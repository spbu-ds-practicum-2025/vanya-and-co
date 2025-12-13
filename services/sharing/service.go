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

	"github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
)

// ShareLink - запись о шаринге
type ShareLink struct {
	Token     string     `json:"token"`
	FileID    string     `json:"fileId"`  // формат: username/filename
	OwnerID   string     `json:"ownerId"` // владелец файла
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Service struct {
	mu      sync.RWMutex
	links   map[string]ShareLink
	cluster *file.ReplicaCluster
	ttl     time.Duration
}

// New создает сервис шаринга
func New(cluster *file.ReplicaCluster, ttl time.Duration) *Service {
	if cluster == nil {
		cluster = file.NewCluster(0)
	}
	return &Service{
		links:   make(map[string]ShareLink),
		cluster: cluster,
		ttl:     ttl,
	}
}

func generateToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Create создает ссылку для общего доступа
// POST /share/create?file=username/filename&owner=username
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("file") // формат: username/filename
	owner := r.URL.Query().Get("owner") // владелец

	if fileID == "" {
		http.Error(w, `{"error": "missing file parameter"}`, http.StatusBadRequest)
		return
	}

	if owner == "" {
		// Пытаемся извлечь owner из fileID (формат: username/filename)
		if !strings.Contains(fileID, "/") {
			http.Error(w, `{"error": "file must be in format username/filename"}`, http.StatusBadRequest)
			return
		}
		owner = strings.Split(fileID, "/")[0]
	}

	token := generateToken()

	now := time.Now().UTC()
	var expires *time.Time
	if s.ttl > 0 {
		t := now.Add(s.ttl)
		expires = &t
	}

	link := ShareLink{
		Token:     token,
		FileID:    fileID,
		OwnerID:   owner,
		ExpiresAt: expires,
		CreatedAt: now,
	}

	s.mu.Lock()
	s.links[token] = link
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token":     token,
		"file":      fileID,
		"owner":     owner,
		"expiresAt": expires,
		"shareUrl":  fmt.Sprintf("/share/%s", token),
	})
}

// Get returns info about a share link (GET /share/{token})
func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/share/"):]
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	link, ok := s.links[token]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error": "share link not found"}`, http.StatusNotFound)
		return
	}

	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(link)
}

// Download позволяет скачать файл по токену
// GET /share/{token}/download
func (s *Service) Download(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/share/"):]
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}

	// remove trailing /download if present
	if strings.HasSuffix(token, "/download") {
		token = strings.TrimSuffix(token, "/download")
	}

	s.mu.RLock()
	link, ok := s.links[token]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error": "share link not found"}`, http.StatusNotFound)
		return
	}

	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
		return
	}

	parts := strings.Split(link.FileID, "/")
	if len(parts) != 2 {
		http.Error(w, `{"error": "invalid file format"}`, http.StatusBadRequest)
		return
	}

	username := parts[0]
	filename := parts[1]

	if s.cluster != nil {
		rel := filepath.Join(username, filename)
		if data, ok := s.cluster.ReadAny(rel); ok {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
			_, _ = w.Write(data)
			return
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/files/download?name=%s&user=%s", filename, username), http.StatusFound)
}

// Revoke отзывает ссылку
// DELETE /share/{token}
func (s *Service) Revoke(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/share/"):]
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	_, ok := s.links[token]
	if ok {
		delete(s.links, token)
	}
	s.mu.Unlock()

	if !ok {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "share link revoked"})
}

// List возвращает все ссылки пользователя
// GET /share/list?owner=username
func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, `{"error": "owner parameter required"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	userLinks := []ShareLink{}
	for _, link := range s.links {
		if link.OwnerID == owner {
			userLinks = append(userLinks, link)
		}
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"count": len(userLinks), "links": userLinks})
}
