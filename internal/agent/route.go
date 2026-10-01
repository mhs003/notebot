package agent

import (
	"encoding/json"
	"strings"

	"github.com/mhs003/notebot/internal/store"
	"github.com/mhs003/notebot/needle"
)

// route interprets common note-management requests deterministically. needle3
// is unreliable at picking the right tool for explicit verbs like "move" or
// "delete", so those are resolved in Go and only ambiguous requests fall
// through to the model.
func (a *Agent) route(prompt string) ([]needle.FunctionCall, bool) {
	if a.store == nil {
		return nil, false
	}
	p := strings.ToLower(strings.TrimSpace(prompt))
	notes, err := a.store.List("", 100)
	if err != nil {
		return nil, false
	}
	folders, _ := a.store.Folders()

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

	// move note: "move <note> to <folder>"
	if verb(p, "move", "put") {
		head, tail, ok := strings.Cut(p, " to ")
		if ok {
			folder := matchFolder(tail, folders)
			if folder == "" {
				folder = normalizeFolder(strings.TrimSuffix(strings.TrimSpace(tail), " folder"))
			}
			if n := bestNote(head, notes); n != nil && folder != "" {
				return call("move_note", map[string]string{"id": n.ID, "to_folder": folder})
			}
		}
	}

	// delete note: "delete the X note"
	if verb(p, "delete", "remove", "trash") && !strings.Contains(p, "folder") {
		if n := bestNote(p, notes); n != nil {
			return call("delete_note", map[string]string{"id": n.ID})
		}
	}

	return nil, false
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
}

// bestNote picks the note whose title shares the most meaningful words with
// the scope text. Ties go to the most recently updated note.
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
	bestScore := 0
	for _, n := range notes {
		score := 0
		for _, w := range strings.Fields(strings.ToLower(n.Title)) {
			w = strings.Trim(w, `.,!?"'()`)
			if words[w] {
				score++
			}
		}
		if score > bestScore {
			bestScore, best = score, n
		}
	}
	return best
}
