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
	NeedsConfirm bool     `json:"needs_confirm"`
	ConfirmID    string   `json:"confirm_id"`
	Summary      string   `json:"summary"`
	Calls        []string `json:"calls"`
}

func newAskCmd(dataDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask <request...>",
		Short: "Natural-language tool calls (create, move, delete, rename, list)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			yes, _ := cmd.Flags().GetBool("yes")
			prompt := strings.Join(args, " ")

			st, cfg, err := loadStore(*dataDir)
			if err != nil {
				return err
			}
			defer st.Close()

			runDaemon := func(p, confirmID string) (*stepResult, bool) {
				body, _ := json.Marshal(map[string]string{"prompt": p, "confirm_id": confirmID})
				req, err := http.NewRequest(http.MethodPost, daemonURL(cfg)+"/api/agent", bytes.NewReader(body))
				if err != nil {
					return nil, false
				}
				req.Header.Set("Content-Type", "application/json")
				if cfg.PasswordHash != "" {
					req.AddCookie(&http.Cookie{Name: "notebot_session", Value: store.SessionToken(cfg.SessionSecret)})
				}
				resp, err := http.DefaultClient.Do(req)
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
			runLocal := func(p, confirmID string) (*stepResult, error) {
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
				res, err := ag.RunStep(ctx, p, confirmID)
				if err != nil {
					return nil, err
				}
				return &stepResult{
					NeedsConfirm: res.NeedsConfirm,
					ConfirmID:    res.ConfirmID,
					Summary:      res.Summary,
					Calls:        res.Calls,
				}, nil
			}
			defer func() {
				if ag != nil {
					ag.Close()
				}
			}()

			run := func(p, confirmID string) (*stepResult, error) {
				if res, ok := runDaemon(p, confirmID); ok {
					return res, nil
				}
				return runLocal(p, confirmID)
			}

			res, err := run(prompt, "")
			if err != nil {
				return err
			}
			fmt.Println(res.Summary)

			if res.NeedsConfirm {
				if !confirm("Apply this change?", yes) {
					fmt.Println("aborted")
					return nil
				}
				res, err = run("", res.ConfirmID)
				if err != nil {
					return err
				}
				fmt.Println(res.Summary)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("yes", "y", false, "auto-confirm destructive actions")
	return cmd
}
