package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/store"
	"github.com/spf13/cobra"
)

type stepResult struct {
	NeedsConfirm bool        `json:"needs_confirm"`
	ConfirmID    string      `json:"confirm_id"`
	Summary      string      `json:"summary"`
	Calls        []string    `json:"calls"`
	NeedsChoice  bool        `json:"needs_choice"`
	ChoiceID     string      `json:"choice_id"`
	Existing     *store.Note `json:"existing"`
	Proposed     *store.Note `json:"proposed"`
}

func newAskCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask <request...>",
		Short: "Natural-language tool calls (create, update, move, delete, rename, list)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			return runAsk(*dataDir, strings.Join(args, " "), yes)
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "auto-confirm destructive actions")
	return cmd
}

// commandVerbs are the leading words that mark a capture as a management
// request rather than a note to save.
var commandVerbs = map[string]bool{
	"update": true, "change": true, "delete": true, "remove": true, "move": true,
}

// looksLikeCommand reports whether text is an explicit note-management request.
func looksLikeCommand(text string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	if len(fields) == 0 {
		return false
	}
	return commandVerbs[strings.Trim(fields[0], `.,!?"'()`)]
}

type askRequest struct {
	Prompt    string `json:"prompt"`
	ConfirmID string `json:"confirm_id"`
	ChoiceID  string `json:"choice_id"`
	Decision  string `json:"decision"`
}

// runAsk routes a natural-language request through the daemon when it is
// reachable, otherwise through an in-process agent.
func runAsk(dataDir, prompt string, yes bool) error {
	st, cfg, err := loadStore(dataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	runDaemon := func(req askRequest) (*stepResult, bool) {
		body, _ := json.Marshal(req)
		httpReq, err := http.NewRequest(http.MethodPost, daemonURL(cfg)+"/api/agent", bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if cfg.PasswordHash != "" {
			httpReq.AddCookie(&http.Cookie{Name: "notebot_session", Value: store.SessionToken(cfg.SessionSecret)})
		}
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			return nil, false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, false
		}
		var res stepResult
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return nil, false
		}
		return &res, true
	}

	var ag *agent.Agent
	runLocal := func(req askRequest) (*stepResult, error) {
		if ag == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			a, err := agent.New(ctx, cfg, st)
			if err != nil {
				return nil, err
			}
			ag = a
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		var res agent.StepResult
		var err error
		if req.ChoiceID != "" {
			res, err = ag.ResolveChoice(ctx, req.ChoiceID, req.Decision)
		} else {
			res, err = ag.RunStep(ctx, req.Prompt, req.ConfirmID)
		}
		if err != nil {
			return nil, err
		}
		return &stepResult{
			NeedsConfirm: res.NeedsConfirm,
			ConfirmID:    res.ConfirmID,
			Summary:      res.Summary,
			Calls:        res.Calls,
			NeedsChoice:  res.NeedsChoice,
			ChoiceID:     res.ChoiceID,
			Existing:     res.Existing,
			Proposed:     res.Proposed,
		}, nil
	}
	defer func() {
		if ag != nil {
			ag.Close()
		}
	}()

	run := func(req askRequest) (*stepResult, error) {
		if res, ok := runDaemon(req); ok {
			return res, nil
		}
		return runLocal(req)
	}

	res, err := run(askRequest{Prompt: prompt})
	if err != nil {
		return err
	}
	fmt.Println(res.Summary)

	// Existing title: ask whether to update it or create a new note.
	if res.NeedsChoice {
		var refined agent.Refined
		if res.Proposed != nil {
			refined = agent.Refined{Title: res.Proposed.Title, Body: res.Proposed.Body, Folder: res.Proposed.Folder, Tags: res.Proposed.Tags}
		}
		decision := "new"
		switch askDuplicate(res.Existing, refined) {
		case choiceCancel:
			decision = "cancel"
		case choiceUpdate:
			decision = "update"
		}
		res, err = run(askRequest{ChoiceID: res.ChoiceID, Decision: decision})
		if err != nil {
			return err
		}
		fmt.Println(res.Summary)
		return nil
	}

	if res.NeedsConfirm {
		if !confirm("Apply this change?", yes) {
			fmt.Println("aborted")
			return nil
		}
		res, err = run(askRequest{ConfirmID: res.ConfirmID})
		if err != nil {
			return err
		}
		fmt.Println(res.Summary)
	}
	return nil
}
