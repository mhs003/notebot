package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/store"
	"golang.org/x/term"
)

// captureChoice is the answer to "an existing note has this title".
type captureChoice int

const (
	choiceUpdate captureChoice = iota
	choiceNew
	choiceCancel
)

// askDuplicate decides what to do when a captured note's title already exists.
// It prompts on a terminal and defaults to creating a new note otherwise, so
// scripts never block.
func askDuplicate(existing *store.Note, r agent.Refined) captureChoice {
	fmt.Printf("\nA note titled %q already exists:\n", existing.Title)
	fmt.Printf("  %s  [%s]  %s\n", existing.ID[:8], existing.Folder, oneLine(existing.Body, 70))
	fmt.Printf("\nNew text:\n  %s\n\n", oneLine(r.Body, 70))

	if !term.IsTerminal(int(syscall.Stdin)) {
		fmt.Println("creating a new note (not interactive; use --new to silence)")
		return choiceNew
	}
	for {
		fmt.Print("  [u]pdate the existing note, [n]ew note, [c]ancel? ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "u", "update":
			return choiceUpdate
		case "n", "new", "":
			return choiceNew
		case "c", "cancel", "q":
			return choiceCancel
		}
	}
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > max {
		s = string([]rune(s)[:max]) + "…"
	}
	if s == "" {
		return "(empty)"
	}
	return s
}
