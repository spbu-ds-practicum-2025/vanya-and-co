package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"vanya-and-co/services/auth"
	"vanya-and-co/services/file"
)

func main() {
	authService := &auth.AuthService{}
	cwd, _ := os.Getwd()
	basePath := filepath.Join(cwd, "services", "file", "data")
	fileService := file.New(basePath, 1000)


	// Главная страница
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "services/gateway/static/index.html")
	})

	// Статика
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("services/gateway/static"))))

	// Auth
	http.HandleFunc("/auth/register", authService.Register)
	http.HandleFunc("/auth/login", authService.Login)

	// File API
	http.HandleFunc("/files/upload", fileService.Upload)
	http.HandleFunc("/files/list", fileService.List)
	http.HandleFunc("/files/download", fileService.Download)
	http.HandleFunc("/files/delete", fileService.Delete)

	log.Println("http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
