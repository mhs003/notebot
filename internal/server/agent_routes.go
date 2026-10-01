package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/store"
)

// applyRefined overwrites a note's content with a refined capture.
func applyRefined(st *store.Store, n *store.Note, r agent.Refined) error {
	n.Title = r.Title
	n.Body = r.Body
	n.Folder = r.Folder
	n.Tags = r.Tags
	return st.Update(n)
}

func (s *Server) AttachAgentRoutes(ag *agent.Agent) {
	s.mux.HandleFunc("/api/capture", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Raw      string         `json:"raw"`
			Decision string         `json:"decision"`  // "", "new", or "update"
			TargetID string         `json:"target_id"` // the note to update
			Refined  *agent.Refined `json:"refined"`   // reuse a prior refinement
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Raw == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()

		var refined agent.Refined
		if body.Refined != nil && body.Refined.Title != "" {
			refined = *body.Refined
		} else {
			var err error
			refined, err = ag.RefineAndRoute(ctx, body.Raw)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		switch body.Decision {
		case "new":
			n := &store.Note{
				Title: refined.Title, Body: refined.Body, Folder: refined.Folder,
				Tags: refined.Tags, Source: "cli", RawPrompt: body.Raw,
			}
			if err := s.st.Create(n); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, n)
		case "update":
			n, err := s.st.Get(body.TargetID)
			if err != nil {
				http.Error(w, "target note not found", http.StatusNotFound)
				return
			}
			if err := applyRefined(s.st, n, refined); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, n)
		default:
			// Ask before creating a second note with the same title.
			if existing, err := s.st.FindSimilarTitle(refined.Title); err == nil && existing != nil {
				writeJSON(w, map[string]any{
					"needs_choice": true,
					"existing":     existing,
					"refined":      refined,
				})
				return
			}
			n := &store.Note{
				Title: refined.Title, Body: refined.Body, Folder: refined.Folder,
				Tags: refined.Tags, Source: "cli", RawPrompt: body.Raw,
			}
			if err := s.st.Create(n); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, n)
		}
	})
	s.mux.HandleFunc("/api/agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Prompt    string `json:"prompt"`
			ConfirmID string `json:"confirm_id"`
			ChoiceID  string `json:"choice_id"`
			Decision  string `json:"decision"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		if body.ChoiceID != "" {
			res, err := ag.ResolveChoice(ctx, body.ChoiceID, body.Decision)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, res)
			return
		}
		res, err := ag.RunStep(ctx, body.Prompt, body.ConfirmID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, res)
	})
	s.mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, map[string]any{
				"port":       s.cfg.Port,
				"data_dir":   s.cfg.DataDir,
				"model_path": s.cfg.ModelPath,
			})
		case http.MethodPut:
			var body struct {
				ModelPath *string `json:"model_path"`
				Port      *int    `json:"port"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			cfg, err := store.LoadConfig(s.cfg.DataDir)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			restart := false
			if body.ModelPath != nil && *body.ModelPath != cfg.ModelPath {
				cfg.ModelPath = *body.ModelPath
				restart = true
			}
			if body.Port != nil {
				cfg.Port = *body.Port
			}
			if err := cfg.Save(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "restart_required": restart})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
