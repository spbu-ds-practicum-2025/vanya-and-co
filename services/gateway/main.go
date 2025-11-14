package main

import (
    "log"
    "net/http"
)

type AuthService struct{}
func (a *AuthService) Register(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("register ok"))
}
func (a *AuthService) Login(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("login ok"))
}

type FileService struct{}
func (f *FileService) Upload(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("upload ok"))
}
func (f *FileService) Download(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("download ok"))
}

func main() {
    authService := &AuthService{}
    fileService := &FileService{}

    http.HandleFunc("/auth/register", authService.Register)
    http.HandleFunc("/auth/login", authService.Login)

    http.HandleFunc("/files/upload", fileService.Upload)
    http.HandleFunc("/files/", fileService.Download)

    log.Println("Server started at http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
