package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"vanya-and-co/services/auth"
	"vanya-and-co/services/auth/storage"
)

func main() {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(cwd))) // move up to repo root when running from cmd/server
	dbPath := filepath.Join(projectRoot, "services", "auth", "data", "users.db")

	store, err := storage.NewSQLite(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	svc := auth.New(store)

	// endpoints
	http.HandleFunc("/auth/login", svc.Login)
	http.HandleFunc("/auth/register", svc.Register)
	http.HandleFunc("/auth/logout", svc.Logout)
	http.HandleFunc("/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
		if user, ok := svc.AuthFromRequest(r); ok {
			w.Write([]byte(user))
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	addr := ":5100"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("auth service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))

}
