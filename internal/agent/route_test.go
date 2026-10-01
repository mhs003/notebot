package agent

import (
	"testing"
	"time"

	"github.com/mhs003/notebot/internal/store"
)

func mkNote(id, title string) *store.Note {
	return &store.Note{ID: id, Title: title, Updated: time.Now()}
}

func TestBestNoteIgnoresStopwords(t *testing.T) {
	notes := []*store.Note{
		mkNote("poem", "Poem about the monsoon"),
		mkNote("milk", "Buy milk and eggs"),
	}
	got := bestNote("move the milk note", notes)
	if got == nil || got.ID != "milk" {
		t.Fatalf("expected milk note, got %+v", got)
	}

	if bestNote("delete the note", notes) != nil {
		t.Error("stopword-only scope should match nothing")
	}
}

func TestFromTo(t *testing.T) {
	cases := []struct{ in, old, new string }{
		{"rename folder weekend to errands", "weekend", "errands"},
		{"rename the archive folder to old", "the archive", "old"},
	}
	for _, c := range cases {
		old, new := fromTo(c.in)
		if old != c.old || new != c.new {
			t.Errorf("%q: got (%q,%q) want (%q,%q)", c.in, old, new, c.old, c.new)
		}
	}
}

func TestVerb(t *testing.T) {
	if !verb("delete the milk note", "delete", "remove") {
		t.Error("expected delete verb")
	}
	if verb("create a note", "delete", "remove") {
		t.Error("did not expect delete verb")
	}
}
