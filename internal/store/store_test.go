package store

import (
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestFindSimilarTitle(t *testing.T) {
	st := newTestStore(t)
	if err := st.Create(&Note{Title: "Chores", Body: "walk the dog"}); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		query string
		want  bool
	}{
		{"chores", true},
		{"Chores tomorrow", true}, // shares the significant word "chores"
		{"nothing like this", false},
	}
	for _, c := range cases {
		got, err := st.FindSimilarTitle(c.query)
		if err != nil {
			t.Fatal(err)
		}
		if (got != nil) != c.want {
			t.Errorf("%q: matched=%v, want %v", c.query, got != nil, c.want)
		}
	}
}

func TestQueryDateRange(t *testing.T) {
	st := newTestStore(t)
	old := &Note{Title: "Old", Created: time.Now().AddDate(0, 0, -10)}
	if err := st.Create(old); err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&Note{Title: "New"}); err != nil {
		t.Fatal(err)
	}

	from := time.Now().AddDate(0, 0, -1)
	notes, err := st.Query(Query{Since: from})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].Title != "New" {
		t.Fatalf("expected only the new note, got %d notes", len(notes))
	}

	all, err := st.Query(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(all))
	}
}

func TestResolveIDPrefix(t *testing.T) {
	st := newTestStore(t)
	n := &Note{Title: "Target"}
	if err := st.Create(n); err != nil {
		t.Fatal(err)
	}

	got, err := st.ResolveID(n.ID[:6])
	if err != nil {
		t.Fatal(err)
	}
	if got != n.ID {
		t.Fatalf("got %q want %q", got, n.ID)
	}

	if _, err := st.ResolveID("zzzzzz"); err == nil {
		t.Error("expected an error for an unknown prefix")
	}
}

func TestDeleteFolderMovesNotesToInbox(t *testing.T) {
	st := newTestStore(t)
	n := &Note{Title: "Filing", Folder: "work"}
	if err := st.Create(n); err != nil {
		t.Fatal(err)
	}

	if err := st.DeleteFolder("work"); err != nil {
		t.Fatal(err)
	}
	got, err := st.Get(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Folder != "inbox" {
		t.Fatalf("note should move to inbox, got %q", got.Folder)
	}
}
