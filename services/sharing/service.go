package sharing

import (
  "encoding/json"
  "fmt"
  "math/rand"
  "net/http"
  "sync"
)

type ShareLink struct {
  Token string `json:"token"`
  FileID string `json:"fileId"`
}

type SharingService struct {
  mu sync.Mutex
  links map[string]ShareLink
}

func New() *SharingService { return &SharingService{links: make(map[string]ShareLink)} }

func (s *SharingService) CreateLink(w http.ResponseWriter, r *http.Request) {
  fid := r.URL.Query().Get("file")
  token := fmt.Sprintf("%06d", rand.Intn(1000000))
  s.mu.Lock()
  s.links[token] = ShareLink{Token: token, FileID: fid}
  s.mu.Unlock()
  json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (s *SharingService) GetFileID(w http.ResponseWriter, r *http.Request) {
  token := r.URL.Query().Get("token")
  s.mu.Lock()
  link, ok := s.links[token]
  s.mu.Unlock()
  if !ok { http.Error(w,"not found",404); return }
  json.NewEncoder(w).Encode(link)
}
