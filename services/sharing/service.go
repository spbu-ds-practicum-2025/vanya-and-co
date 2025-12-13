package sharing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vanya-and-co/services/file"
)

type Link struct {
	Token   string    `json:"token"`
	Owner   string    `json:"owner"`
	File    string    `json:"file"`
	Expires time.Time `json:"expires"`
}

type SharingService struct {
	mu      sync.Mutex
	links   map[string]Link
	cluster *file.ReplicaCluster
	ttl     time.Duration
}

func New(cluster *file.ReplicaCluster, ttl time.Duration) *SharingService {
	if cluster == nil {
		// If nil, create a dummy cluster with no nodes
		cluster = file.NewCluster("", 0)
	}
	s := &SharingService{links: make(map[string]Link), cluster: cluster, ttl: ttl}
	return s
}

func generateToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Create - create a public link
func (s *SharingService) Create(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	fileName := r.URL.Query().Get("file")
	if owner == "" || fileName == "" {
		http.Error(w, "owner or file required", http.StatusBadRequest)
		return
	}
	token := generateToken()
	link := Link{Token: token, Owner: owner, File: fileName, Expires: time.Now().Add(s.ttl)}
	s.mu.Lock()
	s.links[token] = link
	s.mu.Unlock()

	// Return a direct download URL
	w.Write([]byte(fmt.Sprintf("/share/%s/download", token)))
}

// Download by token
func (s *SharingService) Download(w http.ResponseWriter, r *http.Request) {
	// token as last segment before /download or "token" query
	token := r.URL.Query().Get("token")
	if token == "" {
		// try deducing from path
		// expected path: /share/{token}/download
		p := r.URL.Path
		const prefix = "/share/"
		if len(p) > len(prefix) && p[:len(prefix)] == prefix {
			rest := p[len(prefix):]
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) > 0 {
				token = parts[0]
			}
		}
	}
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	link, ok := s.links[token]
	s.mu.Unlock()
	if !ok || time.Now().After(link.Expires) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rel := filepath.Join(filepath.Base(link.Owner), filepath.Base(link.File))
	// read from cluster replicas
	if data, ok := s.cluster.ReadAny(rel); ok {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(link.File)+"\"")
		w.Write(data)
		return
	}
	http.Error(w, "file not found", http.StatusNotFound)
}

// List links for owner
func (s *SharingService) List(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, "owner required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var arr []Link
	for _, l := range s.links {
		if l.Owner == owner {
			arr = append(arr, l)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(arr)
}

// Get - token info
func (s *SharingService) Get(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	link, ok := s.links[token]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(link)
}

// Revoke - delete a link
func (s *SharingService) Revoke(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	delete(s.links, token)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
