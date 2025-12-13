package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type AuthService struct {
	mu       sync.RWMutex
	users    map[string]string // user -> pass
	sessions map[string]string // session -> user
	path     string
}

func New(usersPath string) *AuthService {
	s := &AuthService{users: make(map[string]string), sessions: make(map[string]string), path: usersPath}
	// try load
	f, err := os.Open(usersPath)
	if err == nil {
		defer f.Close()
		_ = json.NewDecoder(f).Decode(&s.users)
	}
	return s
}

func (s *AuthService) save() {
	if s.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	f, err := os.Create(s.path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(s.users)
}

func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	pass := r.URL.Query().Get("pass")
	if user == "" || pass == "" {
		http.Error(w, "user or pass required", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	got, ok := s.users[user]
	s.mu.RUnlock()
	if !ok || got != pass {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sid := generateToken()
	s.mu.Lock()
	s.sessions[sid] = user
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "session", Value: sid, Path: "/"})
	w.Write([]byte(user))
}

func (s *AuthService) Register(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	pass := r.URL.Query().Get("pass")
	if user == "" || pass == "" {
		http.Error(w, "user or pass required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[user]; ok {
		http.Error(w, "user exists", http.StatusBadRequest)
		return
	}
	s.users[user] = pass
	s.save()
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(user))
}

func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "no session", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	delete(s.sessions, c.Value)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/"})
	w.WriteHeader(http.StatusNoContent)
}

func (s *AuthService) AuthFromRequest(r *http.Request) (string, bool) {
	c, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	s.mu.RLock()
	u, ok := s.sessions[c.Value]
	s.mu.RUnlock()
	return u, ok
}

func generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
