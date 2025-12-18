package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
)

func requireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	u := r.Header.Get("X-User")
	if u == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return u, true
}

func main() {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(cwd)))
	base := filepath.Join(projectRoot, "services", "file", "data")
	svc := filepkg.New(base, 10)

	http.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		if u, ok := requireUser(w, r); ok {
			svc.Upload(w, r, u)
		}
	})

	http.HandleFunc("/files/list", func(w http.ResponseWriter, r *http.Request) {
		if u, ok := requireUser(w, r); ok {
			svc.List(w, r, u)
		}
	})

	http.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		if u, ok := requireUser(w, r); ok {
			// if 'user' param is provided and different, treat as download of other's file
			if other := r.URL.Query().Get("user"); other != "" && other != u {
				svc.Download(w, r, other)
				return
			}
			svc.Download(w, r, u)
		}
	})

	http.HandleFunc("/files/delete", func(w http.ResponseWriter, r *http.Request) {
		if u, ok := requireUser(w, r); ok {
			svc.Delete(w, r, u)
		}
	})

	addr := ":5200"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("file service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
