package sharing

import (
	"database/sql"
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

type Link struct {
	Token   string    `json:"token"`
	Owner   string    `json:"owner"`
	File    string    `json:"file"`
	Expires time.Time `json:"expires"`
}

type SharingService struct {
	mu      sync.Mutex
	links   map[string]Link
	cluster *filepkg.ReplicaCluster
	db      *sql.DB
	ttl     time.Duration
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
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS links (token TEXT PRIMARY KEY, owner TEXT, file TEXT, expires INTEGER);`)
	}
	s := &SharingService{links: make(map[string]Link), cluster: cluster, ttl: ttl, db: db}
	return s
}

// Create - create a public link
func (s *SharingService) Create(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	fileName := r.URL.Query().Get("file")
	ttlStr := r.URL.Query().Get("ttl") // optional ttl in seconds
	var ttl time.Duration
	if ttlStr != "" {
		if v, err := time.ParseDuration(ttlStr); err == nil {
			ttl = v
		}
	}
	if owner == "" || fileName == "" {
		http.Error(w, "owner or file required", http.StatusBadRequest)
		return
	}
	token := generateToken()
	exp := time.Now().Add(s.ttl)
	if ttl != 0 {
		exp = time.Now().Add(ttl)
	}
	link := Link{Token: token, Owner: owner, File: fileName, Expires: exp}
	s.mu.Lock()
	s.links[token] = link
	s.mu.Unlock()
	if s.db != nil {
		_, _ = s.db.Exec(`INSERT INTO links (token, owner, file, expires) VALUES (?, ?, ?, ?)`, token, owner, fileName, exp.Unix())
	}

	// Return JSON with direct download URL and expiry
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"url": fmt.Sprintf("/share/%s/download", token), "expires": exp.Unix()})
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
	var link Link
	var ok bool
	if s.db != nil {
		var exp int64
		if err := s.db.QueryRow(`SELECT owner, file, expires FROM links WHERE token = ?`, token).Scan(&link.Owner, &link.File, &exp); err == nil {
			link.Token = token
			link.Expires = time.Unix(exp, 0)
			ok = true
		}
	} else {
		s.mu.Lock()
		link, ok = s.links[token]
		s.mu.Unlock()
	}
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
	var arr []Link
	if s.db != nil {
		rows, err := s.db.Query(`SELECT token, file, expires FROM links WHERE owner = ?`, owner)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var t string
				var f string
				var e int64
				if rows.Scan(&t, &f, &e) == nil {
					arr = append(arr, Link{Token: t, Owner: owner, File: f, Expires: time.Unix(e, 0)})
				}
			}
		}
	} else {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, l := range s.links {
			if l.Owner == owner {
				arr = append(arr, l)
			}
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
	var link Link
	if s.db != nil {
		var exp int64
		if err := s.db.QueryRow(`SELECT owner, file, expires FROM links WHERE token = ?`, token).Scan(&link.Owner, &link.File, &exp); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		} else {
			link.Token = token
			link.Expires = time.Unix(exp, 0)
		}
	} else {
		s.mu.Lock()
		var ok bool
		link, ok = s.links[token]
		s.mu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
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
	if s.db != nil {
		_, _ = s.db.Exec(`DELETE FROM links WHERE token = ?`, token)
	}
	w.WriteHeader(http.StatusNoContent)
}
