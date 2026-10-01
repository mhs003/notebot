package store

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func removeDirIfEmpty(base, name string) error {
	dir := filepath.Join(base, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}

func (s *Store) Update(n *Note) error {
	cur, err := s.Get(n.ID)
	if err != nil {
		return err
	}
	oldPath := cur.FilePath(s.dataDir)
	n.Created = cur.Created
	n.Updated = time.Now().UTC()
	if n.Folder == "" {
		n.Folder = cur.Folder
	}
	path := n.FilePath(s.dataDir)
	if err := writeFile(path, n.Marshal()); err != nil {
		return err
	}
	if path != oldPath {
		_ = os.Remove(oldPath)
	}
	tags := strings.Join(n.Tags, ",")
	if _, err := s.db.Exec(`UPDATE notes SET title=?,folder=?,body=?,tags=?,updated=?,path=? WHERE id=?`,
		n.Title, n.Folder, n.Body, tags, n.Updated.Format(time.RFC3339), path, n.ID); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM notes_fts WHERE id=?`, n.ID); err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO notes_fts (id,title,body,folder) VALUES (?,?,?,?)`,
		n.ID, n.Title, n.Body, n.Folder)
	return err
}

func (s *Store) Delete(id string) error {
	cur, err := s.Get(id)
	if err != nil {
		return err
	}
	_ = os.Remove(cur.FilePath(s.dataDir))
	if _, err := s.db.Exec(`DELETE FROM notes WHERE id=?`, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM notes_fts WHERE id=?`, id); err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM embeddings WHERE doc_id=?`, id)
	return err
}

func (s *Store) Move(id, folder string) error {
	n, err := s.Get(id)
	if err != nil {
		return err
	}
	n.Folder = folder
	if err := s.Update(n); err != nil {
		return err
	}
	return s.CreateFolder(folder)
}

func (s *Store) RenameFolder(old, new string) error {
	notes, err := s.List(old, 0)
	if err != nil {
		return err
	}
	for _, n := range notes {
		n.Folder = new
		if err := s.Update(n); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`DELETE FROM folders WHERE name=?`, old); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(NotesDir(s.dataDir), old))
	return s.CreateFolder(new)
}

func (s *Store) SaveEmbedding(docID string, vec []float32) error {
	buf := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	_, err := s.db.Exec(`INSERT INTO embeddings (doc_id,dim,vec) VALUES (?,?,?)
		ON CONFLICT(doc_id) DO UPDATE SET dim=excluded.dim, vec=excluded.vec`,
		docID, len(vec), buf)
	return err
}

func (s *Store) LoadEmbedding(docID string) ([]float32, error) {
	var dim int
	var buf []byte
	if err := s.db.QueryRow(`SELECT dim,vec FROM embeddings WHERE doc_id=?`, docID).Scan(&dim, &buf); err != nil {
		return nil, err
	}
	if len(buf) != dim*4 {
		return nil, fmt.Errorf("store: corrupt embedding for %s", docID)
	}
	out := make([]float32, dim)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return out, nil
}

func Cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		if i >= len(b) {
			break
		}
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
