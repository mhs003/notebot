package server

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/mhs003/notebot/internal/store"
	"golang.org/x/crypto/bcrypt"
)

var WebFS fs.FS

type Server struct {
	cfg store.Config
	st  *store.Store
	mux *http.ServeMux

	sessMu  sync.Mutex
	rotated string
}

func New(cfg store.Config, st *store.Store) *Server {
	s := &Server{cfg: cfg, st: st}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/auth/login", s.login)
	mux.HandleFunc("/api/auth/logout", s.logout)
	mux.HandleFunc("/api/auth/status", s.authStatus)
	mux.HandleFunc("/api/notes", s.notes)
	mux.HandleFunc("/api/notes/", s.noteByID)
	mux.HandleFunc("/api/folders", s.folders)
	mux.HandleFunc("/api/recent", s.recent)
	mux.HandleFunc("/api/search", s.search)
	s.mux = mux
	return s
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/status" {
			s.mux.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if !s.authed(r) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			s.mux.ServeHTTP(w, r)
			return
		}
		if WebFS == nil {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><html><body style="font-family:sans-serif;padding:40px"><h1>Notebot</h1><p>Dashboard not built yet. Run <code>make web</code> inside the project to build web/dist, then restart notebotd.</p><p><a href="/api/health">health</a></p></body></html>`)
			return
		}
		s.serveSPA(w, r)
	})
}

func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	ctype := map[string]string{
		".html": "text/html", ".js": "application/javascript", ".css": "text/css",
		".json": "application/json", ".svg": "image/svg+xml", ".png": "image/png",
		".ico": "image/x-icon",
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path != "" {
		if st, err := fs.Stat(WebFS, path); err == nil && !st.IsDir() {
			f, err := WebFS.Open(path)
			if err == nil {
				defer f.Close()
				if ext := extOf(path); ext != "" {
					if ct, ok := ctype[ext]; ok {
						w.Header().Set("Content-Type", ct)
					}
				}
				http.ServeContent(w, r, path, st.ModTime(), f.(io.ReadSeeker))
				return
			}
		}
	}
	f, err := WebFS.Open("index.html")
	if err != nil {
		http.Error(w, "no index", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "no index", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	http.ServeContent(w, r, "index.html", info.ModTime(), f.(io.ReadSeeker))
}

func extOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '.' {
			return p[i:]
		}
		if p[i] == '/' {
			return ""
		}
	}
	return ""
}

// token is the expected session cookie value, derived from a persistent
// secret so sessions survive daemon restarts and page refreshes.
func (s *Server) token() string {
	s.sessMu.Lock()
	secret := s.cfg.SessionSecret
	if s.rotated != "" {
		secret = s.rotated
	}
	s.sessMu.Unlock()
	return store.SessionToken(secret)
}

func (s *Server) authed(r *http.Request) bool {
	if s.cfg.PasswordHash == "" {
		return true
	}
	want := s.token()
	if want == "" {
		return false
	}
	c, err := r.Cookie("notebot_session")
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(want))
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}
