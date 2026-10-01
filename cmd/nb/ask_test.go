package main

import "testing"

func TestLooksLikeCommand(t *testing.T) {
	commands := []string{
		"update the Chores note with: something",
		"Delete the old plan",
		"move the milk note to shopping",
	}
	for _, c := range commands {
		if !looksLikeCommand(c) {
			t.Errorf("%q should be treated as a command", c)
		}
	}

	notes := []string{
		"I will do my chores tomorrow",
		"reminder: buy milk",
		"today i wanted to write a poem",
		"the meeting moved to friday",
	}
	for _, n := range notes {
		if looksLikeCommand(n) {
			t.Errorf("%q should be captured as a note", n)
		}
	}
}
