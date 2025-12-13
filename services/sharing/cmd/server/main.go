package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"vanya-and-co/services/file"
	"vanya-and-co/services/sharing"
)

func main() {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(cwd)))
	base := filepath.Join(projectRoot, "services", "file", "data")

	cluster := file.NewCluster(base, 3)
	svc := sharing.New(cluster, 7*24*time.Hour)

	http.HandleFunc("/share/create", svc.Create)
	http.HandleFunc("/share/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == "GET" && (len(path) > 7 && path[len(path)-9:] == "/download"):
			svc.Download(w, r)
		case r.Method == "GET" && (path == "/share/" || path == "/share/list"):
			svc.List(w, r)
		case r.Method == "GET":
			svc.Get(w, r)
		case r.Method == "DELETE":
			svc.Revoke(w, r)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	addr := ":5300"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("sharing service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
