package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Folder    string    `json:"folder"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags"`
	Source    string    `json:"source"`
	RawPrompt string    `json:"raw_prompt,omitempty"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func NotesDir(dataDir string) string {
	return filepath.Join(dataDir, "notes")
}

func (n *Note) FilePath(dataDir string) string {
	folder := n.Folder
	if folder == "" {
		folder = "inbox"
	}
	slug := Slug(n.Title)
	if slug == "" {
		slug = "note"
	}
	if len(slug) > 48 {
		slug = slug[:48]
	}
	return filepath.Join(NotesDir(dataDir), folder, fmt.Sprintf("%s-%s.md", slug, n.ID))
}

func (n *Note) Marshal() string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", n.ID)
	fmt.Fprintf(&b, "title: %s\n", quote(n.Title))
	fmt.Fprintf(&b, "folder: %s\n", quote(n.Folder))
	fmt.Fprintf(&b, "created: %s\n", n.Created.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "updated: %s\n", n.Updated.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(quoted(n.Tags), ", "))
	fmt.Fprintf(&b, "source: %s\n", quote(n.Source))
	if n.RawPrompt != "" {
		fmt.Fprintf(&b, "raw_prompt: %s\n", quote(n.RawPrompt))
	}
	b.WriteString("---\n\n")
	b.WriteString(n.Body)
	if !strings.HasSuffix(n.Body, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func quote(s string) string {
	if strings.ContainsAny(s, ":\n\"'[]{}") || strings.TrimSpace(s) != s || s == "" {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func quoted(ss []string) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, quote(s))
	}
	return out
}

func EnsureDirs(dataDir string) error {
	for _, d := range []string{
		NotesDir(dataDir),
		filepath.Join(NotesDir(dataDir), "inbox"),
		filepath.Join(dataDir, "models"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
