package sharing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/spbu-ds-practicum-2025/vanya-and-co.git/services/file"
)

// FileChecker — минимальный интерфейс для проверки существования файла.
// Мы используем ReplicaCluster через этот интерфейс.
type FileChecker interface {
	// Exists возвращает true, если файл с таким id есть в кластере.
	Exists(fileID string) bool
}

type memFileChecker struct {
	cluster *file.ReplicaCluster
}

func (m *memFileChecker) Exists(fileID string) bool {
	_, ok := m.cluster.ReadAny(fileID)
	return ok
}

// ShareLink - in-memory share record
type ShareLink struct {
	Token     string     `json:"token"`
	FileID    string     `json:"fileId"`
	OwnerID   string     `json:"ownerId,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type Service struct {
	mu         sync.Mutex
	links      map[string]ShareLink
	fileChecker FileChecker
	ttl        time.Duration
}

// NewInMemory creates in-memory sharing service.
// cluster — указатель на ReplicaCluster для проверки существования файла.
// ttl — время жизни ссылки (0 = без истечения).
func NewInMemory(cluster *file.ReplicaCluster, ttl time.Duration) *Service {
	return &Service{
		links:      make(map[string]ShareLink),
		fileChecker: &memFileChecker{cluster: cluster},
		ttl:        ttl,
	}
}

// Create creates a share link: POST /share/create?file=<id>&owner=<ownerID>
// returns {"token":"...","expiresAt": "..."}
func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	fileID := r.URL.Query().Get("file")
	owner := r.URL.Query().Get("owner") // optional owner id
	if fileID == "" {
		http.Error(w, "missing file parameter", http.StatusBadRequest)
		return
	}

	// проверяем, существует ли файл в FileService / ReplicaCluster
	if !s.fileChecker.Exists(fileID) {
		http.Error(w, "file not found", http.StatusNotFound)
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
	_ = json.NewEncoder(w).Encode(link)
}

// Get returns link by token: GET /share/get?token=...
func (s *Service) Get(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	link, ok := s.links[token]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "share link not found", http.StatusNotFound)
		return
	}

	// проверяем TTL
	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		// автоматическое удаление просроченной ссылки
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		http.Error(w, "share link expired", http.StatusGone)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(link)
}

// Revoke revokes token: DELETE /share/revoke?token=...
func (s *Service) Revoke(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token parameter", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	_, ok := s.links[token]
	if ok {
		delete(s.links, token)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func generateToken() string {
	// 8 chars URL-safe tokens
	b := make([]byte, 6)
	now := time.Now().UnixNano()
	// простая псевдо-энтропия: комбинация времени и rand — OK для MVP
	r := (now % 1000000)
	for i := range b {
		b[i] = byte((r>>uint(i*5))&0x3F) + 48
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())[:12]
}


