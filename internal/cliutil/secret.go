package cliutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// ReadPassword reads a password from the terminal without echo, falling back
// to a plain line read when stdin is not a TTY (pipes, tests).
func ReadPassword(prompt string) string {
	fmt.Print(prompt)
	if term.IsTerminal(int(syscall.Stdin)) {
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	r := bufio.NewReader(os.Stdin)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
