package agent

import (
	"encoding/json"
	"strings"

	"github.com/mhs003/notebot/internal/store"
	"github.com/mhs003/notebot/needle"
)

// RoutePrompt reports whether an explicit management request can be resolved
// against the store without consulting the model. Callers use it to decide
// whether text is a command or a note to capture.
func RoutePrompt(st *store.Store, prompt string) bool {
	_, ok := route(st, prompt)
	return ok
}

// route interprets common note-management requests deterministically. needle3
// is unreliable at picking the right tool for explicit verbs like "move" or
// "delete", so those are resolved in Go and only ambiguous requests fall
// through to the model.
func route(st *store.Store, prompt string) ([]needle.FunctionCall, bool) {
	if st == nil {
		return nil, false
	}
	p := strings.ToLower(strings.TrimSpace(prompt))
	notes, err := st.List("", 100)
	if err != nil {
		return nil, false
	}
	folders, _ := st.Folders()

	call := func(name string, args any) ([]needle.FunctionCall, bool) {
		raw, _ := json.Marshal(args)
		return []needle.FunctionCall{{Name: name, Arguments: raw}}, true
	}

	// rename folder: "rename folder A to B" / "rename A to B"
	if verb(p, "rename") && strings.Contains(p, "folder") {
		old, new := fromTo(p)
		if old != "" && new != "" {
			if oldFolder := matchFolder(old, folders); oldFolder != "" {
				return call("rename_folder", map[string]string{"old": oldFolder, "new": normalizeFolder(new)})
			}
		}
	}

	// move note: "move <note> to <folder>". The folder must already exist, or
	// be explicit ("to the recipes folder"), so a plain sentence that happens
	// to start with "move" is not mistaken for a command.
	if verb(p, "move", "put") {
		head, tail, ok := strings.Cut(p, " to ")
		if ok {
			folder := matchFolder(tail, folders)
			if folder == "" && strings.Contains(tail, "folder") {
				folder = normalizeFolder(strings.ReplaceAll(tail, "folder", ""))
			}
			if folder != "" {
				if n := bestNote(head, notes); n != nil {
					return call("move_note", map[string]string{"id": n.ID, "to_folder": folder})
				}
			}
		}
	}

	// delete note: "delete the X note"
	if verb(p, "delete", "remove", "trash") && !strings.Contains(p, "folder") {
		if n := bestNote(p, notes); n != nil {
			return call("delete_note", map[string]string{"id": n.ID})
		}
	}

	// update note: "update the X note with: <text>" / "change the X note to <text>"
	if verb(p, "update", "change", "edit") {
		content, cut := updateTarget(prompt)
		if cut >= 0 && content != "" {
			if n := bestNote(prompt[:cut], notes); n != nil {
				return call("update_note", map[string]string{"id": n.ID, "content": content})
			}
		}
	}

	return nil, false
}

// updateTarget splits a request into the new value and the byte index where it
// starts, using separators such as "with:" and "to".
func updateTarget(s string) (string, int) {
	low := strings.ToLower(s)
	seps := []string{" with:", " content:", " body:", " to:", " with ", " content to ", " body to ", " to "}
	best, sepLen := -1, 0
	for _, sep := range seps {
		if i := strings.Index(low, sep); i >= 0 && (best == -1 || i < best) {
			best, sepLen = i, len(sep)
		}
	}
	if best == -1 {
		return "", -1
	}
	content := strings.TrimSpace(s[best+sepLen:])
	for _, pfx := range []string{"say ", "that ", "read "} {
		content = strings.TrimPrefix(content, pfx)
	}
	return strings.TrimSpace(content), best
}

func verb(s string, words ...string) bool {
	for _, w := range words {
		for _, f := range strings.Fields(s) {
			if strings.Trim(f, `.,!?"'()`) == w {
				return true
			}
		}
	}
	return false
}

func fromTo(s string) (string, string) {
	_, rest, ok := strings.Cut(s, "rename")
	if !ok {
		return "", ""
	}
	rest = strings.TrimSpace(rest)
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "folder"))
	old, next, ok := strings.Cut(rest, " to ")
	if !ok {
		return "", ""
	}
	old = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(old), "folder"))
	return old, strings.TrimSpace(next)
}

// matchFolder returns the existing folder named in s, if any.
func matchFolder(s string, folders []string) string {
	s = strings.ToLower(s)
	for _, f := range folders {
		if strings.Contains(s, strings.ToLower(f)) {
			return f
		}
	}
	return ""
}

// routeStop lists words that carry no note-identifying signal.
var routeStop = map[string]bool{
	"the": true, "a": true, "an": true, "to": true, "into": true, "from": true,
	"my": true, "and": true, "of": true, "for": true, "about": true, "with": true,
	"note": true, "notes": true, "folder": true, "file": true,
	"move": true, "put": true, "delete": true, "remove": true, "trash": true,
	"rename": true, "list": true, "show": true, "create": true, "add": true,
	"update": true, "change": true, "edit": true, "save": true,
}

// bestNote picks the note whose title best matches the scope text: most
// distinct shared words wins, then the title with the highest precision
// (shared words over title length), then the most recently updated. The
// precision tie-break stops a long title that happens to repeat a keyword
// from beating a short, exact one.
func bestNote(scope string, notes []*store.Note) *store.Note {
	words := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(scope)) {
		w = strings.Trim(w, `.,!?"'()`)
		if len(w) >= 3 && !routeStop[w] {
			words[w] = true
		}
	}
	if len(words) == 0 {
		return nil
	}
	var best *store.Note
	bestScore, bestPrec := 0, 0.0
	for _, n := range notes {
		titleWords := strings.Fields(strings.ToLower(n.Title))
		matched := map[string]bool{}
		for _, w := range titleWords {
			w = strings.Trim(w, `.,!?"'()`)
			if words[w] {
				matched[w] = true
			}
		}
		score := len(matched)
		if score == 0 {
			continue
		}
		prec := float64(score) / float64(len(titleWords))
		if score > bestScore || (score == bestScore && prec > bestPrec) {
			bestScore, bestPrec, best = score, prec, n
		}
	}
	return best
}
