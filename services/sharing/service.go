package sharing

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	_ "modernc.org/sqlite"
)

// ShareLink - запись о шаринге
type ShareLink struct {
	Token     string     `json:"token"`
	Owner     string     `json:"owner"`
	File      string     `json:"file"`      // формат: username/filename
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

// FileChecker — интерфейс для проверки существования файла
type FileChecker interface {
	Exists(relPath string) bool
}

// clusterFileChecker - адаптер для ReplicaCluster
type clusterFileChecker struct {
	cluster *filepkg.ReplicaCluster
}

func (c *clusterFileChecker) Exists(relPath string) bool {
	_, ok := c.cluster.ReadAny(relPath)
	return ok
}

type SharingService struct {
	mu          sync.RWMutex
	links       map[string]ShareLink
	cluster     *filepkg.ReplicaCluster
	fileChecker FileChecker
	db          *sql.DB
	defaultTTL  time.Duration
}

func New(cluster *filepkg.ReplicaCluster, ttl time.Duration) *SharingService {
	if cluster == nil {
		// If nil, create a dummy cluster with no nodes
		cluster = filepkg.NewCluster("", 0)
	}
	
	// open db in sharing/data/share.db next to repo when running from cmd/server
	cwd, _ := os.Getwd()
	dbPath := filepath.Join(cwd, "services", "sharing", "data", "sharing.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	db, _ := sql.Open("sqlite", dbPath)
	if db != nil {
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS links (token TEXT PRIMARY KEY, owner TEXT, file TEXT, expires INTEGER, created INTEGER);`)
	}
	
	s := &SharingService{
		links:       make(map[string]ShareLink),
		cluster:     cluster,
		fileChecker: &clusterFileChecker{cluster: cluster},
		db:          db,
		defaultTTL:  ttl,
	}
	
	// Load existing links from database
	s.loadFromDB()
	
	return s
}

// loadFromDB загружает существующие ссылки из базы данных
func (s *SharingService) loadFromDB() {
	if s.db == nil {
		return
	}
	
	rows, err := s.db.Query(`SELECT token, owner, file, expires, created FROM links`)
	if err != nil {
		return
	}
	defer rows.Close()
	
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for rows.Next() {
		var token, owner, file string
		var expiresInt, createdInt int64
		var expires *int64
		
		if err := rows.Scan(&token, &owner, &file, &expires, &createdInt); err != nil {
			continue
		}
		
		var expiresAt *time.Time
		if expires != nil {
			t := time.Unix(*expires, 0)
			expiresAt = &t
		}
		
		createdAt := time.Unix(createdInt, 0)
		
		s.links[token] = ShareLink{
			Token:     token,
			Owner:     owner,
			File:      file,
			ExpiresAt: expiresAt,
			CreatedAt: createdAt,
		}
	}
}

// Create - create a public link
func (s *SharingService) Create(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	fileName := r.URL.Query().Get("file")
	ttlStr := r.URL.Query().Get("ttl") // optional ttl in seconds
	
	if owner == "" || fileName == "" {
		// Пытаемся извлечь owner из file (формат: username/filename)
		if fileName != "" && strings.Contains(fileName, "/") {
			parts := strings.SplitN(fileName, "/", 2)
			if owner == "" && len(parts) == 2 {
				owner = parts[0]
				fileName = parts[1]
			}
		}
		
		if owner == "" || fileName == "" {
			http.Error(w, `{"error": "owner and file required"}`, http.StatusBadRequest)
			return
		}
	}
	
	fileID := owner + "/" + fileName
	
	// Проверяем, существует ли файл
	if !s.fileChecker.Exists(fileID) {
		http.Error(w, `{"error": "file not found"}`, http.StatusNotFound)
		return
	}
	
	// Определяем TTL
	var ttl time.Duration
	if ttlStr != "" {
		if v, err := time.ParseDuration(ttlStr); err == nil {
			ttl = v
		}
	}
	if ttl == 0 {
		ttl = s.defaultTTL
	}
	
	token := generateToken()
	now := time.Now().UTC()
	var expiresAt *time.Time
	
	if ttl > 0 {
		exp := now.Add(ttl)
		expiresAt = &exp
	}
	
	link := ShareLink{
		Token:     token,
		Owner:     owner,
		File:      fileName,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	
	s.mu.Lock()
	s.links[token] = link
	s.mu.Unlock()
	
	// Сохраняем в базу данных
	if s.db != nil {
		var expiresInt *int64
		if expiresAt != nil {
			val := expiresAt.Unix()
			expiresInt = &val
		}
		
		_, _ = s.db.Exec(
			`INSERT INTO links (token, owner, file, expires, created) VALUES (?, ?, ?, ?, ?)`,
			token, owner, fileName, expiresInt, now.Unix(),
		)
	}
	
	// Return JSON with direct download URL and expiry
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token":     token,
		"file":      fileName,
		"owner":     owner,
		"expiresAt": expiresAt,
		"shareUrl":  fmt.Sprintf("/share/%s", token),
		"downloadUrl": fmt.Sprintf("/share/%s/download", token),
	})
}

// Get - token info
func (s *SharingService) Get(w http.ResponseWriter, r *http.Request) {
	// Извлекаем token из URL пути
	path := r.URL.Path
	const prefix = "/share/"
	
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, `{"error": "invalid path"}`, http.StatusBadRequest)
		return
	}
	
	token := path[len(prefix):]
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}
	
	// Удаляем "/download" из токена если есть
	if strings.HasSuffix(token, "/download") {
		token = token[:len(token)-len("/download")]
	}
	
	var link ShareLink
	var ok bool
	
	if s.db != nil {
		var fileStr, ownerStr string
		var expiresInt *int64
		var createdInt int64
		
		err := s.db.QueryRow(
			`SELECT owner, file, expires, created FROM links WHERE token = ?`,
			token,
		).Scan(&ownerStr, &fileStr, &expiresInt, &createdInt)
		
		if err == nil {
			var expiresAt *time.Time
			if expiresInt != nil {
				t := time.Unix(*expiresInt, 0)
				expiresAt = &t
			}
			
			link = ShareLink{
				Token:     token,
				Owner:     ownerStr,
				File:      fileStr,
				ExpiresAt: expiresAt,
				CreatedAt: time.Unix(createdInt, 0),
			}
			ok = true
		}
	} else {
		s.mu.RLock()
		link, ok = s.links[token]
		s.mu.RUnlock()
	}
	
	if !ok {
		http.Error(w, `{"error": "share link not found"}`, http.StatusNotFound)
		return
	}
	
	// Проверяем срок действия
	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		
		if s.db != nil {
			_, _ = s.db.Exec(`DELETE FROM links WHERE token = ?`, token)
		}
		
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(link)
}

// Download by token
func (s *SharingService) Download(w http.ResponseWriter, r *http.Request) {
	// Извлекаем token из URL пути
	path := r.URL.Path
	const prefix = "/share/"
	
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, `{"error": "invalid path"}`, http.StatusBadRequest)
		return
	}
	
	token := path[len(prefix):]
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}
	
	// Удаляем "/download" из токена если есть
	if strings.HasSuffix(token, "/download") {
		token = token[:len(token)-len("/download")]
	}
	
	var link ShareLink
	var ok bool
	
	if s.db != nil {
		var fileStr, ownerStr string
		var expiresInt *int64
		var createdInt int64
		
		err := s.db.QueryRow(
			`SELECT owner, file, expires, created FROM links WHERE token = ?`,
			token,
		).Scan(&ownerStr, &fileStr, &expiresInt, &createdInt)
		
		if err == nil {
			var expiresAt *time.Time
			if expiresInt != nil {
				t := time.Unix(*expiresInt, 0)
				expiresAt = &t
			}
			
			link = ShareLink{
				Token:     token,
				Owner:     ownerStr,
				File:      fileStr,
				ExpiresAt: expiresAt,
				CreatedAt: time.Unix(createdInt, 0),
			}
			ok = true
		}
	} else {
		s.mu.RLock()
		link, ok = s.links[token]
		s.mu.RUnlock()
	}
	
	if !ok {
		http.Error(w, `{"error": "share link not found"}`, http.StatusNotFound)
		return
	}
	
	// Проверяем срок действия
	if link.ExpiresAt != nil && time.Now().UTC().After(*link.ExpiresAt) {
		s.mu.Lock()
		delete(s.links, token)
		s.mu.Unlock()
		
		if s.db != nil {
			_, _ = s.db.Exec(`DELETE FROM links WHERE token = ?`, token)
		}
		
		http.Error(w, `{"error": "share link expired"}`, http.StatusGone)
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
	
	http.Error(w, `{"error": "file not found"}`, http.StatusNotFound)
}

// List links for owner
func (s *SharingService) List(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	if owner == "" {
		http.Error(w, `{"error": "owner required"}`, http.StatusBadRequest)
		return
	}
	
	var arr []ShareLink
	if s.db != nil {
		rows, err := s.db.Query(`SELECT token, file, expires, created FROM links WHERE owner = ?`, owner)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var token, file string
				var expires *int64
				var created int64
				if rows.Scan(&token, &file, &expires, &created) == nil {
					var expiresAt *time.Time
					if expires != nil {
						t := time.Unix(*expires, 0)
						expiresAt = &t
					}
					
					arr = append(arr, ShareLink{
						Token:     token,
						Owner:     owner,
						File:      file,
						ExpiresAt: expiresAt,
						CreatedAt: time.Unix(created, 0),
					})
				}
			}
		}
	} else {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for _, l := range s.links {
			if l.Owner == owner {
				arr = append(arr, l)
			}
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"count": len(arr),
		"links": arr,
	})
}

// Revoke - delete a link
func (s *SharingService) Revoke(w http.ResponseWriter, r *http.Request) {
	// Извлекаем token из URL пути
	path := r.URL.Path
	const prefix = "/share/"
	
	if !strings.HasPrefix(path, prefix) {
		http.Error(w, `{"error": "invalid path"}`, http.StatusBadRequest)
		return
	}
	
	token := path[len(prefix):]
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	
	if token == "" {
		http.Error(w, `{"error": "token required"}`, http.StatusBadRequest)
		return
	}
	
	s.mu.Lock()
	_, ok := s.links[token]
	delete(s.links, token)
	s.mu.Unlock()
	
	if s.db != nil {
		_, _ = s.db.Exec(`DELETE FROM links WHERE token = ?`, token)
	}
	
	if !ok {
		http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "share link revoked",
	})
}

// generateToken генерирует безопасный токен
func generateToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}