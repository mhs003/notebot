package agent

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	spaceRe  = regexp.MustCompile(`[ \t]+`)
	nlRe     = regexp.MustCompile(`\n{3,}`)
	spacePun = regexp.MustCompile(`\s+([,.!?;:])`)
)

// cleanText normalizes a raw capture: trims, collapses whitespace, fixes
// spacing before punctuation, capitalizes the first letter, and ensures the
// text ends with a sentence terminator.
func cleanText(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	s = spacePun.ReplaceAllString(s, "$1")
	s = spaceRe.ReplaceAllString(s, " ")
	s = nlRe.ReplaceAllString(s, "\n\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	s = strings.Join(lines, "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	for i, c := range r {
		if unicode.IsSpace(c) {
			continue
		}
		r[i] = unicode.ToUpper(c)
		break
	}
	s = string(r)
	last := s[len(s)-1]
	if !strings.ContainsRune(".!?\"')]", rune(last)) {
		s += "."
	}
	return s
}

// deriveTitle builds a title from the first line or sentence when the model
// does not supply a usable one.
func deriveTitle(raw string) string {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "Untitled"
	}
	if i := strings.IndexAny(t, "\n.!?"); i > 0 {
		t = t[:i]
	}
	t = strings.TrimSpace(t)
	if len([]rune(t)) > 60 {
		t = string([]rune(t)[:60])
		if i := strings.LastIndexByte(t, ' '); i > 0 {
			t = t[:i]
		}
	}
	t = trimTrailingStopwords(t)
	if t == "" {
		t = strings.TrimSpace(raw)
	}
	r := []rune(t)
	for i, c := range r {
		r[i] = unicode.ToUpper(c)
		break
	}
	return strings.TrimSpace(string(r))
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "to": true,
	"for": true, "with": true, "of": true, "in": true, "on": true, "at": true,
	"by": true, "is": true, "was": true, "will": true, "i": true,
}

func trimTrailingStopwords(s string) string {
	words := strings.Fields(s)
	for len(words) > 1 && stopwords[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

var folderKeywords = []struct {
	folder string
	words  []string
}{
	{"poems", []string{"poem", "poetry", "haiku", "verse", "sonnet"}},
	{"books", []string{"book", "library", "author", "novel", "read"}},
	{"shopping", []string{"buy", "shop", "purchase", "order", "cart", "groceries"}},
	{"work", []string{"deadline", "project", "meeting", "task", "client", "deploy", "ticket"}},
	{"ideas", []string{"idea", "concept", "thought", "brainstorm"}},
}

// routeFolder picks a folder deterministically: an existing folder whose name
// appears in the text wins, then keyword matches, then "inbox".
func routeFolder(raw string, existing []string) string {
	lower := strings.ToLower(raw)
	for _, f := range existing {
		if f == "inbox" || f == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(f)) {
			return f
		}
	}
	for _, k := range folderKeywords {
		for _, w := range k.words {
			if strings.Contains(lower, w) {
				return k.folder
			}
		}
	}
	return "inbox"
}

func normalizeFolder(f string) string {
	f = strings.ToLower(strings.TrimSpace(f))
	f = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(f, "-")
	return strings.Trim(f, "-")
}
