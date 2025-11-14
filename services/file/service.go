package file

import (
	"io"
	"net/http"
	"sync"
	"time"
)

type FileEntry struct {
	Data []byte
}

type FileService struct {
	mu sync.RWMutex
	store map[string]FileEntry
}

func New() *FileService { return &FileService{store: make(map[string]FileEntry)} }

func (s *FileService) Upload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	data, _ := io.ReadAll(r.Body)
	go func() {
		time.Sleep(300 * time.Millisecond)
		s.mu.Lock()
		s.store[id] = FileEntry{Data: data}
		s.mu.Unlock()
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (s *FileService) Download(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	s.mu.RLock()
	v, ok := s.store[id]
	s.mu.RUnlock()
	if !ok {
		http.Error(w, "not found", 404)
		return
	}
	w.Write(v.Data)
}