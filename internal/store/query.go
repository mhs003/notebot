package store

import (
	"fmt"
	"strings"
	"time"
)

// Query filters and orders notes. Zero-valued time bounds are ignored.
type Query struct {
	Folder    string
	Since     time.Time
	Until     time.Time
	ByUpdated bool
	Limit     int
	Oldest    bool
}

func (q Query) column() string {
	if q.ByUpdated {
		return "updated"
	}
	return "created"
}

// Query returns notes matching the filter.
func (s *Store) Query(q Query) ([]*Note, error) {
	var (
		conds []string
		args  []any
	)
	if q.Folder != "" {
		conds = append(conds, "folder = ?")
		args = append(args, q.Folder)
	}
	col := q.column()
	if !q.Since.IsZero() {
		conds = append(conds, col+" >= ?")
		args = append(args, q.Since.UTC().Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		conds = append(conds, col+" <= ?")
		args = append(args, q.Until.UTC().Format(time.RFC3339))
	}

	sqlStr := `SELECT id,title,folder,body,tags,source,raw_prompt,created,updated FROM notes`
	if len(conds) > 0 {
		sqlStr += " WHERE " + strings.Join(conds, " AND ")
	}
	if q.Oldest {
		sqlStr += " ORDER BY " + col + " ASC"
	} else {
		sqlStr += " ORDER BY " + col + " DESC"
	}
	if q.Limit > 0 {
		sqlStr += fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

// SearchQuery combines full-text search with a filter. If the FTS5 MATCH
// syntax rejects the query it falls back to a LIKE scan so odd input still
// returns something useful.
func (s *Store) SearchQuery(query string, q Query) ([]*Note, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.Query(q)
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}

	sqlStr := `SELECT n.id,n.title,n.folder,n.body,n.tags,n.source,n.raw_prompt,n.created,n.updated
		FROM notes_fts f JOIN notes n ON n.id = f.id
		WHERE notes_fts MATCH ?`
	args := []any{ftsQuery(query)}
	if q.Folder != "" {
		sqlStr += " AND n.folder = ?"
		args = append(args, q.Folder)
	}
	col := "n." + q.column()
	if !q.Since.IsZero() {
		sqlStr += " AND " + col + " >= ?"
		args = append(args, q.Since.UTC().Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		sqlStr += " AND " + col + " <= ?"
		args = append(args, q.Until.UTC().Format(time.RFC3339))
	}
	sqlStr += " ORDER BY " + col + " DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(sqlStr, args...)
	if err == nil {
		defer rows.Close()
		notes, err := scanNotes(rows)
		if err == nil {
			return notes, nil
		}
	} else if rows != nil {
		rows.Close()
	}

	return s.likeSearch(query, q, limit)
}

func (s *Store) likeSearch(query string, q Query, limit int) ([]*Note, error) {
	sqlStr := `SELECT id,title,folder,body,tags,source,raw_prompt,created,updated FROM notes
		WHERE (title LIKE ? OR body LIKE ?)`
	like := "%" + query + "%"
	args := []any{like, like}
	if q.Folder != "" {
		sqlStr += " AND folder = ?"
		args = append(args, q.Folder)
	}
	col := q.column()
	if !q.Since.IsZero() {
		sqlStr += " AND " + col + " >= ?"
		args = append(args, q.Since.UTC().Format(time.RFC3339))
	}
	if !q.Until.IsZero() {
		sqlStr += " AND " + col + " <= ?"
		args = append(args, q.Until.UTC().Format(time.RFC3339))
	}
	sqlStr += " ORDER BY " + col + " DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotes(rows)
}

// ftsQuery wraps each token in double quotes so punctuation and operators in
// user input cannot break the MATCH expression.
func ftsQuery(s string) string {
	tokens := strings.Fields(s)
	quoted := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, "")
		if t == "" {
			continue
		}
		quoted = append(quoted, `"`+t+`"`)
	}
	if len(quoted) == 0 {
		return `""`
	}
	return strings.Join(quoted, " ")
}

// titleStopWords carry no identifying signal when comparing titles.
var titleStopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "of": true, "to": true,
	"for": true, "with": true, "my": true, "is": true, "in": true, "on": true,
	"about": true, "note": true, "notes": true,
}

func titleWords(s string) []string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return ' '
	}, s)
	var out []string
	for _, w := range strings.Fields(s) {
		if len(w) >= 3 && !titleStopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

func jaccard(a, b []string) float64 {
	set := func(xs []string) map[string]bool {
		m := map[string]bool{}
		for _, x := range xs {
			m[x] = true
		}
		return m
	}
	sa, sb := set(a), set(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for k := range sa {
		if sb[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(sa)+len(sb)-inter)
}

// FindSimilarTitle returns the existing note whose title is most similar to
// the given one, or nil when nothing is close enough. "Chores" and "Chores
// tomorrow" match; unrelated titles do not.
func (s *Store) FindSimilarTitle(title string) (*Note, error) {
	want := titleWords(title)
	if len(want) == 0 {
		return nil, nil
	}
	notes, err := s.List("", 500)
	if err != nil {
		return nil, err
	}
	var best *Note
	bestSim := 0.0
	for _, n := range notes {
		sim := jaccard(want, titleWords(n.Title))
		if sim > bestSim {
			bestSim, best = sim, n
		}
	}
	if bestSim >= 0.5 {
		return best, nil
	}
	return nil, nil
}

// ResolveID turns a full id or a unique prefix into a full id.
func (s *Store) ResolveID(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", fmt.Errorf("store: empty id")
	}
	if _, err := s.Get(prefix); err == nil {
		return prefix, nil
	}
	rows, err := s.db.Query(`SELECT id FROM notes WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var matches []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		matches = append(matches, id)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("store: no note matches id %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("store: id %q is ambiguous (%d matches)", prefix, len(matches))
	}
}

// DeleteFolder moves any notes in the folder to inbox, then removes the
// folder record and its directory.
func (s *Store) DeleteFolder(name string) error {
	if name == "" || name == "inbox" {
		return fmt.Errorf("store: cannot delete %q", name)
	}
	notes, err := s.List(name, 0)
	if err != nil {
		return err
	}
	for _, n := range notes {
		n.Folder = "inbox"
		if err := s.Update(n); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`DELETE FROM folders WHERE name=?`, name); err != nil {
		return err
	}
	return removeDirIfEmpty(NotesDir(s.dataDir), name)
}

func scanNotes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*Note, error) {
	var out []*Note
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
