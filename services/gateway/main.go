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

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.Write([]byte(`
            <!DOCTYPE html>
            <html>
            <head>
                <meta charset="utf-8">
                <title>Cloud Storage</title>
            </head>
            <body>
                <h1>Cloud Storage Project</h1>
                <p>Сервер работает!</p>
                <ul>
                    <li><a href="/auth/register">Регистрация</a></li>
                    <li><a href="/auth/login">Логин</a></li>
                    <li><a href="/files/upload">Загрузка</a></li>
                    <li><a href="/files/download">Выгрузка</a></li>
                </ul>
            </body>
            </html>
        `))
    })


    http.HandleFunc("/auth/register", authService.Register)
    http.HandleFunc("/auth/login", authService.Login)

    http.HandleFunc("/files/upload", fileService.Upload)
    http.HandleFunc("/files/download", fileService.Download)

    log.Println("Server started at http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
