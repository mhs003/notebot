package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/mhs003/notebot/internal/agent"
	"github.com/mhs003/notebot/internal/store"
)

func (s *Server) AttachAgentRoutes(ag *agent.Agent) {
	s.mux.HandleFunc("/api/capture", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.authed(r) && r.Host != "" {
		}
		var body struct {
			Raw string `json:"raw"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Raw == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		n, err := ag.Capture(ctx, body.Raw)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, n)
	})
	s.mux.HandleFunc("/api/agent", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Prompt    string `json:"prompt"`
			ConfirmID string `json:"confirm_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
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
