package agent

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mhs003/notebot/internal/store"
	"github.com/mhs003/notebot/needle"
)

//go:embed tools.json
var toolsJSON string

type Agent struct {
	mu    sync.Mutex
	n     *needle.Needle
	store *store.Store
	cfg   store.Config
}

func New(ctx context.Context, cfg store.Config, st *store.Store) (*Agent, error) {
	weights := store.ResolveWeightsPath(cfg)
	if _, err := statFile(weights); err != nil {
		return nil, fmt.Errorf("agent: model not found at %s (copy models/needle3.cact there or set model_path in config.json): %w", weights, err)
	}
	n, err := needle.New(ctx, needle.Config{
		WeightsPath: weights,
		ToolsJSON:   toolsJSON,
		SystemPrompt: fmt.Sprintf(
			"You manage notes. Choose the tool that matches the request: "+
				"create_note to save a new note; update_note to change one; "+
				"delete_note to remove one; move_note to change a note's folder; "+
				"create_folder for a new folder; rename_folder to rename one; "+
				"list_notes to show notes. "+
				"For a rough capture with no explicit action, call refine_text. "+
				"When a tool needs an id, copy the exact id from the provided note list. "+
				"Date: %s. Destructive actions need confirmation.",
			time.Now().Format("2006-01-02")),
	})
	if err != nil {
		return nil, err
	}
	return &Agent{n: n, store: st, cfg: cfg}, nil
}

func (a *Agent) Close() error { return a.n.Close() }

type Refined struct {
	Title  string
	Body   string
	Folder string
	Tags   []string
}

// RefineAndRoute cleans the raw capture deterministically and uses needle3 to
// suggest a title and folder. The body is always the cleaned user text —
// needle3 does not rewrite prose, so we never trust its content field.
func (a *Agent) RefineAndRoute(ctx context.Context, raw string) (Refined, error) {
	body := cleanText(raw)

	var title, folderHint string
	var tags []string

	a.mu.Lock()
	_ = a.n.Reset(ctx)
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	res, err := a.n.CompleteResult(cctx, raw, 256)
	cancel()
	a.mu.Unlock()

	if err == nil {
		for _, c := range res.FunctionCalls {
			if c.Name != "refine_text" && c.Name != "create_note" {
				continue
			}
			var p struct {
				Title      string   `json:"title"`
				FolderHint string   `json:"folder_hint"`
				Tags       []string `json:"tags"`
			}
			if c.Bind(&p) == nil {
				title = strings.TrimSpace(p.Title)
				folderHint = p.FolderHint
				tags = p.Tags
			}
			break
		}
	}

	if len([]rune(title)) < 3 || strings.EqualFold(title, raw) {
		title = deriveTitle(raw)
	}

	var existing []string
	if a.store != nil {
		existing, _ = a.store.Folders()
	}
	folder := normalizeFolder(folderHint)
	if folder == "" || folder == "inbox" {
		folder = routeFolder(raw, existing)
	}

	return Refined{Title: title, Body: body, Folder: folder, Tags: tags}, nil
}

type StepResult struct {
	NeedsConfirm bool     `json:"needs_confirm"`
	ConfirmID    string   `json:"confirm_id,omitempty"`
	Summary      string   `json:"summary"`
	Calls        []string `json:"calls"`
}

func (a *Agent) Capture(ctx context.Context, raw string) (*store.Note, error) {
	r, err := a.RefineAndRoute(ctx, raw)
	if err != nil {
		return nil, err
	}
	n := &store.Note{Title: r.Title, Body: r.Body, Folder: r.Folder, Tags: r.Tags, Source: "cli", RawPrompt: raw}
	if err := a.store.Create(n); err != nil {
		return nil, err
	}
	return n, nil
}

type PendingCall struct {
	Name string
	Args json.RawMessage
}

var pending = struct {
	sync.Mutex
	m map[string]PendingCall
}{m: map[string]PendingCall{}}

func (a *Agent) RunStep(ctx context.Context, prompt string, confirmID string) (StepResult, error) {
	if confirmID != "" {
		pending.Lock()
		p, ok := pending.m[confirmID]
		if ok {
			delete(pending.m, confirmID)
		}
		pending.Unlock()
		if !ok {
			return StepResult{}, fmt.Errorf("agent: unknown confirm_id")
		}
		if err := a.exec(ctx, p.Name, p.Args); err != nil {
			return StepResult{}, err
		}
		return StepResult{Summary: "confirmed: " + p.Name, Calls: []string{p.Name}}, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	calls, ok := a.route(prompt)
	if !ok {
		_ = a.n.Reset(ctx)
		res, err := a.n.CompleteResult(ctx, a.withContext(prompt), 512)
		if err != nil {
			return StepResult{}, err
		}
		calls = res.FunctionCalls
	}

	var out StepResult
	for _, c := range calls {
		out.Calls = append(out.Calls, c.Name)
		if isDestructive(c.Name) {
			id := store.NewID()
			pending.Lock()
			pending.m[id] = PendingCall{Name: c.Name, Args: c.Arguments}
			pending.Unlock()
			out.NeedsConfirm = true
			out.ConfirmID = id
			out.Summary = "confirm " + c.Name + ": " + string(c.Arguments)
			break
		}
		if err := a.exec(ctx, c.Name, c.Arguments); err != nil {
			out.Summary += "error " + c.Name + ": " + err.Error() + "; "
		} else {
			out.Summary += c.Name + " ok; "
		}
	}
	if out.Summary == "" {
		out.Summary = "no tool calls"
	}
	return out, nil
}

// withContext prefixes the request with candidate notes so the model can
// resolve names the user mentions into ids. Candidates are pre-filtered by
// title-word overlap to keep the prompt small and the choice unambiguous.
func (a *Agent) withContext(prompt string) string {
	if a.store == nil {
		return prompt
	}
	all, err := a.store.List("", 100)
	if err != nil || len(all) == 0 {
		return prompt
	}
	// Only add context when the request actually references an existing note;
	// dumping every note into a plain "create" request makes the model copy
	// unrelated content.
	candidates := matchNotes(prompt, all)
	if len(candidates) == 0 {
		return prompt
	}

	var b strings.Builder
	if len(candidates) == 1 {
		b.WriteString("The note being referenced is:\n")
	} else {
		b.WriteString("Candidate notes (choose the matching id):\n")
	}
	for _, n := range candidates {
		fmt.Fprintf(&b, "- id=%s title=%q folder=%q\n", n.ID, n.Title, n.Folder)
	}
	if folders, err := a.store.Folders(); err == nil && len(folders) > 0 {
		fmt.Fprintf(&b, "Existing folders: %s\n", strings.Join(folders, ", "))
	}
	b.WriteString("\nRequest: ")
	b.WriteString(prompt)
	return b.String()
}

// matchNotes returns notes whose title shares a meaningful word with the
// prompt. Empty when nothing matches.
func matchNotes(prompt string, notes []*store.Note) []*store.Note {
	words := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(prompt)) {
		w = strings.Trim(w, `.,!?"'()`)
		if len(w) >= 4 {
			words[w] = true
		}
	}
	if len(words) == 0 {
		return nil
	}
	var out []*store.Note
	for _, n := range notes {
		for _, w := range strings.Fields(strings.ToLower(n.Title)) {
			w = strings.Trim(w, `.,!?"'()`)
			if words[w] {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

func isDestructive(name string) bool {
	switch name {
	case "delete_note", "move_note", "rename_folder":
		return true
	}
	return false
}

func (a *Agent) exec(ctx context.Context, name string, raw json.RawMessage) error {
	switch name {
	case "create_note":
		var p struct {
			Title      string   `json:"title"`
			Content    string   `json:"content"`
			FolderHint string   `json:"folder_hint"`
			Tags       []string `json:"tags"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		f := p.FolderHint
		if f == "" {
			f = "inbox"
		}
		return a.store.Create(&store.Note{Title: p.Title, Body: p.Content, Folder: f, Tags: p.Tags, Source: "agent"})
	case "update_note":
		var p struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		n, err := a.store.Get(p.ID)
		if err != nil {
			return err
		}
		if p.Title != "" {
			n.Title = p.Title
		}
		if p.Content != "" {
			n.Body = p.Content
		}
		return a.store.Update(n)
	case "delete_note":
		var p struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		return a.store.Delete(p.ID)
	case "create_folder":
		var p struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		return a.store.CreateFolder(normalizeFolder(p.Name))
	case "rename_folder":
		var p struct {
			Old string `json:"old"`
			New string `json:"new"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		return a.store.RenameFolder(p.Old, p.New)
	case "move_note":
		var p struct {
			ID       string `json:"id"`
			ToFolder string `json:"to_folder"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		return a.store.Move(p.ID, p.ToFolder)
	case "list_notes":
		return nil
	case "refine_text":
		return nil
	}
	return fmt.Errorf("agent: unknown tool %s", name)
}
