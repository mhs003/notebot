package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mhs003/notebot/internal/dates"
	"github.com/mhs003/notebot/internal/store"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.cfg.PasswordHash == "" || !checkPassword(s.cfg.PasswordHash, body.Password) {
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return
	}
	s.sessMu.Lock()
	s.rotated = ""
	s.sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "notebot_session", Value: s.token(), Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600,
	})
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{
		"authed":       s.authed(r),
		"password_set": s.cfg.PasswordHash != "",
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.sessMu.Lock()
	s.rotated = randomHex(16)
	s.sessMu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "notebot_session", Value: "", Path: "/", MaxAge: -1,
	})
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) notes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		q := store.Query{Folder: r.URL.Query().Get("folder")}
		if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil {
			q.Limit = v
		}
		q.ByUpdated = r.URL.Query().Get("by") == "updated"
		now := time.Now()
		if s := r.URL.Query().Get("since"); s != "" {
			t, err := dates.ParseInstant(s, now, false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			q.Since = t
		}
		if s := r.URL.Query().Get("until"); s != "" {
			t, err := dates.ParseInstant(s, now, true)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			q.Until = t
		}
		notes, err := s.st.Query(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if notes == nil {
			notes = []*store.Note{}
		}
		writeJSON(w, notes)
	case http.MethodPost:
		var n store.Note
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if n.Source == "" {
			n.Source = "web"
		}
		if err := s.st.Create(&n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, &n)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) noteByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notes/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		n, err := s.st.Get(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, n)
	case http.MethodPatch, http.MethodPut:
		n, err := s.st.Get(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var patch struct {
			Title  *string `json:"title"`
			Body   *string `json:"content"`
			Folder *string `json:"folder"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if patch.Title != nil {
			n.Title = *patch.Title
		}
		if patch.Body != nil {
			n.Body = *patch.Body
		}
		if patch.Folder != nil {
			n.Folder = *patch.Folder
		}
		if err := s.st.Update(n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, n)
	case http.MethodDelete:
		if err := s.st.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) folders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		folders, err := s.st.Folders()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if folders == nil {
			folders = []string{}
		}
		writeJSON(w, folders)
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		n := &store.Note{Title: "Welcome", Folder: body.Name, Source: "web"}
		if err := s.st.Create(n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"name": body.Name})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) folderByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/folders/")
	if name == "" {
		http.Error(w, "missing folder name", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPatch, http.MethodPut:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if err := s.st.RenameFolder(name, body.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"name": body.Name})
	case http.MethodDelete:
		if err := s.st.DeleteFolder(name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) recent(w http.ResponseWriter, r *http.Request) {
	notes, err := s.st.Recent(30)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if notes == nil {
		notes = []*store.Note{}
	}
	writeJSON(w, notes)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, []*store.Note{})
		return
	}
	notes, err := s.st.Search(q, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if notes == nil {
		notes = []*store.Note{}
	}
	writeJSON(w, notes)
}
