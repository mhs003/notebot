package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	dataDir string
}

func Open(dataDir string) (*Store, error) {
	if err := EnsureDirs(dataDir); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dataDir+"/index.db")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notes (
			id TEXT PRIMARY KEY, title TEXT NOT NULL, folder TEXT NOT NULL DEFAULT 'inbox',
			body TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '', raw_prompt TEXT NOT NULL DEFAULT '',
			created TEXT NOT NULL, updated TEXT NOT NULL, path TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(id, title, body, folder)`,
		`CREATE TABLE IF NOT EXISTS embeddings (doc_id TEXT PRIMARY KEY, dim INTEGER NOT NULL, vec BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS folders (name TEXT PRIMARY KEY, created TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_folder ON notes(folder)`,
		`CREATE INDEX IF NOT EXISTS idx_notes_updated ON notes(updated DESC)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *Store) Create(n *Note) error {
	if n.ID == "" {
		n.ID = NewID()
	}
	now := time.Now().UTC()
	if n.Created.IsZero() {
		n.Created = now
	}
	n.Updated = now
	if n.Folder == "" {
		n.Folder = "inbox"
	}
	path := n.FilePath(s.dataDir)
	if err := writeFile(path, n.Marshal()); err != nil {
		return err
	}
	tags := strings.Join(n.Tags, ",")
	_, err := s.db.Exec(`INSERT INTO notes (id,title,folder,body,tags,source,raw_prompt,created,updated,path)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		n.ID, n.Title, n.Folder, n.Body, tags, n.Source, n.RawPrompt,
		n.Created.Format(time.RFC3339), n.Updated.Format(time.RFC3339), path)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO notes_fts (id,title,body,folder) VALUES (?,?,?,?)`,
		n.ID, n.Title, n.Body, n.Folder)
	if err != nil {
		return err
	}
	return s.CreateFolder(n.Folder)
}

func (s *Store) Get(id string) (*Note, error) {
	row := s.db.QueryRow(`SELECT id,title,folder,body,tags,source,raw_prompt,created,updated FROM notes WHERE id=?`, id)
	return scanNote(row)
}

func (s *Store) List(folder string, limit int) ([]*Note, error) {
	q := `SELECT id,title,folder,body,tags,source,raw_prompt,created,updated FROM notes`
	var args []any
	if folder != "" {
		q += ` WHERE folder=?`
		args = append(args, folder)
	}
	q += ` ORDER BY updated DESC`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

func (s *Store) Recent(limit int) ([]*Note, error) { return s.List("", limit) }

func (s *Store) Folders() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM (
			SELECT DISTINCT folder AS name FROM notes
			UNION SELECT name FROM folders
		) ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CreateFolder(name string) error {
	if name == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(NotesDir(s.dataDir), name), 0o755); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO folders (name, created) VALUES (?, ?)`,
		name, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *Store) Search(query string, limit int) ([]*Note, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT n.id,n.title,n.folder,n.body,n.tags,n.source,n.raw_prompt,n.created,n.updated
		FROM notes_fts f JOIN notes n ON n.id=f.id WHERE notes_fts MATCH ? LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

type scanner interface{ Scan(dest ...any) error }

func scanNote(r scanner) (*Note, error) {
	var n Note
	var tags, created, updated string
	if err := r.Scan(&n.ID, &n.Title, &n.Folder, &n.Body, &tags, &n.Source, &n.RawPrompt, &created, &updated); err != nil {
		return nil, err
	}
	if tags != "" {
		n.Tags = strings.Split(tags, ",")
	}
	n.Created, _ = time.Parse(time.RFC3339, created)
	n.Updated, _ = time.Parse(time.RFC3339, updated)
	return &n, nil
}
