package sharing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"vanya-and-co/services/file"
)

// FileChecker — интерфейс для проверки существования файла
type FileChecker interface {
	Exists(relPath string) bool
}

// memFileChecker - адаптер для ReplicaCluster
type memFileChecker struct {
	cluster *file.ReplicaCluster
}

func (m *memFileChecker) Exists(relPath string) bool {
	_, ok := m.cluster.ReadAny(relPath)
	return ok
}

// ShareLink - запись о шаринге
type ShareLink struct {
	Token     string     `json:"token"`
	FileID    string     `json:"fileId"`     // формат: username/filename
	OwnerID   string     `json:"ownerId"`    // владелец файла
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Service struct {
	mu          sync.RWMutex
	links       map[string]ShareLink
	fileChecker FileChecker
	ttl         time.Duration
}

// New создает сервис шаринга
func New(cluster *file.ReplicaCluster, ttl time.Duration) *Service {
	return &Service{
		links:       make(map[string]ShareLink),
		fileChecker: &memFileChecker{cluster: cluster},
		ttl:         ttl,
	}
}

// Create создает ссылку для общего доступа
// POST /share/create?file=username/filename&owner=username
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("file")    // формат: username/filename
	owner := r.URL.Query().Get("owner")    // владелец

	if fileID == "" {
		http.Error(w, `{"error": "missing file parameter"}`, http.StatusBadRequest)
		return
	}

	if owner == "" {
		// Пытаемся извлечь owner из fileID (формат: username/filename)
		if len(fileID) < 3 || len(fileID.Split("/")) < 2 {
			http.Error(w, `{"error": "file must be in format username/filename"}`, http.StatusBadRequest)
			return
		}
		owner = strings.Split(fileID, "/")[0]
	}

	// Проверяем, существует ли файл
	if !s.fileChecker.Exists(fileID) {
		http.Error(w, `{"error": "file not found"}`, http.StatusNotFound)
		return
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":     token,
		"file":      fileID,
		"owner":     owner,
		"expiresAt": expires,
		"shareUrl":  fmt.Sprintf("/share/%s", token),
	})
}

// Get возвращает информацию о ссылке
// GET /share/:token
func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем token из URL пути
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

	// Проверяем срок действия
	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(link)
}

// Download позволяет скачать файл по токену
// GET /share/:token/download
func (s *Service) Download(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Path[len("/share/"):]
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}

	// Удаляем "/download" из пути если есть
	if len(token) > 8 && token[len(token)-8:] == "/download" {
		token = token[:len(token)-8]
	}

	s.mu.RLock()
	link, ok := s.links[token]
	s.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error": "share link not found"}`, http.StatusNotFound)
		return
	}

	// Проверяем срок действия
	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
		return
	}

	// Редирект на скачивание файла
	// Формат: /files/download?name=filename&user=username
	parts := strings.Split(link.FileID, "/")
	if len(parts) != 2 {
		http.Error(w, `{"error": "invalid file format"}`, http.StatusBadRequest)
		return
	}
	
	username := parts[0]
	filename := parts[1]
	
	http.Redirect(w, r, fmt.Sprintf("/files/download?name=%s&user=%s", filename, username), http.StatusFound)
}

// Revoke отзывает ссылку
// DELETE /share/:token
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
	json.NewEncoder(w).Encode(map[string]string{
		"message": "share link revoked",
	})
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(userLinks),
		"links": userLinks,
	})
}

// generateToken генерирует безопасный токен
func generateToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}