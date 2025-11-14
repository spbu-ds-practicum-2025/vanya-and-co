package auth

import (
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "net/http"
  "sync"
)

type User struct {
  Username string `json:"username"`
  Password string `json:"password"`
}

type AuthService struct {
  mu sync.Mutex
  users map[string]string
}

func New() *AuthService {
  return &AuthService{users: make(map[string]string)}
}

func (s *AuthService) Register(w http.ResponseWriter, r *http.Request) {
  var u User
  if err := json.NewDecoder(r.Body).Decode(&u); err != nil {http.Error(w,"bad",400); return }
  h := sha256.Sum256([]byte(u.Password))
  s.mu.Lock()
  defer s.mu.Unlock()
  if _, ok := s.users[u.Username]; ok { http.Error(w,"exists",400); return }
  s.users[u.Username] = hex.EncodeToString(h[:])
  w.WriteHeader(http.StatusCreated)
}
