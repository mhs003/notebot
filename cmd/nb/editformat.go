package main

import (
	"strings"

	"github.com/mhs003/notebot/internal/store"
)

// parseEdited reads the simple "title: …\nfolder: …\n---\nbody" format produced
// by editInEditor. The header block is optional.
func parseEdited(orig *store.Note, raw string) *store.Note {
	out := *orig
	body := raw
	if strings.HasPrefix(raw, "title:") || strings.HasPrefix(raw, "folder:") {
		if i := strings.Index(raw, "\n---\n"); i >= 0 {
			header := raw[:i]
			body = strings.TrimLeft(raw[i+len("\n---\n"):], "\n")
			for _, line := range strings.Split(header, "\n") {
				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				val = strings.TrimSpace(val)
				switch strings.ToLower(strings.TrimSpace(key)) {
				case "title":
					if val != "" {
						out.Title = val
					}
				case "folder":
					if val != "" {
						out.Folder = val
					}
				}
			}
		}
	}
	out.Body = strings.TrimRight(body, "\n")
	return &out
}
