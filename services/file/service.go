package file

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type FileEntry struct {
	Data []byte
}

type FileService struct {
	mu    sync.RWMutex
	store map[string]FileEntry // key: user/filename
	base  string
}

// New returns a disk-backed file service
func New(base string, _ int) *FileService {
	f := &FileService{store: make(map[string]FileEntry), base: base}
	_ = os.MkdirAll(base, 0o755)
	return f
}

// New (no args) used by ReplicaCluster to create in-memory nodes
func NewNode() *FileService {
	return &FileService{store: make(map[string]FileEntry)}
}

// Upload expects multipart form field "file"
func (s *FileService) Upload(w http.ResponseWriter, r *http.Request, user string) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	filePart, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer filePart.Close()
	data, _ := io.ReadAll(filePart)
	// write to disk if base provided
	if s.base != "" {
		dir := filepath.Join(s.base, user)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, header.Filename), data, 0o644)
	}
	key := filepath.Join(user, header.Filename)
	s.mu.Lock()
	s.store[key] = FileEntry{Data: data}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"name": header.Filename})
}

func (s *FileService) List(w http.ResponseWriter, r *http.Request, user string) {
	files := []string{}
	if s.base != "" {
		dir := filepath.Join(s.base, user)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, e.Name())
			}
		}
	} else {
		s.mu.RLock()
		for k := range s.store {
			if filepath.Dir(k) == user {
				files = append(files, filepath.Base(k))
			}
		}
		s.mu.RUnlock()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(files)
}

func (s *FileService) Download(w http.ResponseWriter, r *http.Request, user string) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if s.base != "" {
		p := filepath.Join(s.base, user, name)
		data, err := os.ReadFile(p)
		if err == nil {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
			_, _ = w.Write(data)
			return
		}
	}
	// fallback to in-memory store
	key := filepath.Join(user, name)
	s.mu.RLock()
	e, ok := s.store[key]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	_, _ = w.Write(e.Data)
}

func (s *FileService) Delete(w http.ResponseWriter, r *http.Request, user string) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	if s.base != "" {
		_ = os.Remove(filepath.Join(s.base, user, name))
	}
	key := filepath.Join(user, name)
	s.mu.Lock()
	delete(s.store, key)
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}
