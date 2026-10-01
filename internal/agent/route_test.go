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

func TestBestNotePrefersExactShortTitle(t *testing.T) {
	notes := []*store.Note{
		mkNote("long", "Update the Chores note, update with: I will do my chores"),
		mkNote("short", "Chores"),
	}
	got := bestNote("update the Chores note, update", notes)
	if got == nil || got.ID != "short" {
		t.Fatalf("expected the short exact title, got %+v", got)
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

func TestUpdateTarget(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		hasSep bool
	}{
		{"update the Chores note, update with: I will do my chores tomorrow", "I will do my chores tomorrow", true},
		{"change the Chores note to say buy milk", "buy milk", true},
		{"update the chores note with: walk the dog", "walk the dog", true},
		{"update the chores note", "", false},
	}
	for _, c := range cases {
		got, cut := updateTarget(c.in)
		if (cut >= 0) != c.hasSep {
			t.Errorf("%q: separator presence = %v, want %v", c.in, cut >= 0, c.hasSep)
			continue
		}
		if c.hasSep && got != c.want {
			t.Errorf("%q: got %q, want %q", c.in, got, c.want)
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
