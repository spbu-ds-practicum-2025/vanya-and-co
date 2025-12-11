package main

import (
	"log"
	"net/http"

	"vanya-and-co/services/auth"
	"vanya-and-co/services/file"
)


func main() {
	authService := &auth.AuthService{}
	fileService := file.New("services/file/data", 1000)

	http.Handle("/", http.FileServer(http.Dir("services/gateway/static")))

	http.HandleFunc("/auth/register", authService.Register)
	http.HandleFunc("/auth/login", authService.Login)

	http.HandleFunc("/files/upload", fileService.Upload)
	http.HandleFunc("/files/list", fileService.List)
	http.HandleFunc("/files/download", fileService.Download)
    http.HandleFunc("/files/delete", fileService.Delete)

	log.Println("http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
