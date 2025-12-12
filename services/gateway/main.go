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
	// 1. Инициализация сервисов
	cwd, _ := os.Getwd()
	authPath := filepath.Join(cwd, "services", "auth", "data", "users.json")
	authService := auth.New(authPath)
	// Устанавливаем лимит, например, 10 МБ
	basePath := filepath.Join(cwd, "services", "file", "data")
	fileService := file.New(basePath, 10) 

	// Вспомогательная функция для проверки авторизации (middleware)
	requireAuth := func(w http.ResponseWriter, r *http.Request) (username string, ok bool) {
		username, ok = authService.AuthFromRequest(r)
		if !ok {
			// Если не авторизован, перекидываем на логин (Условие 2)
			http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
		}
		return username, ok
	}
	
	// 2. Обработчики Авторизации
	http.HandleFunc("/auth/login", authService.Login)
	http.HandleFunc("/auth/register", authService.Register)
	http.HandleFunc("/auth/logout", authService.Logout)

	// 3. Обработчики Статики
	// Предполагаем, что статика лежит в папке `static`
	staticDir := filepath.Join(cwd, "static")
	// Если папки статики нет, создаем ее для index.html, login-form.html и т.д.
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		_ = os.MkdirAll(staticDir, 0o755)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	// 4. Главная страница
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Если пользователь авторизован, сразу кидаем на файлы
		if _, ok := authService.AuthFromRequest(r); ok {
			http.Redirect(w, r, "/files/list", http.StatusSeeOther)
			return
		}
		// Иначе показываем главную (или редиректим на логин)
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	// 5. Обработчики Файлов (с обязательной авторизацией)
	
	http.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.Upload(w, r, username)
		}
	})

	http.HandleFunc("/files/list", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.List(w, r, username)
		}
	})
	
	http.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.Download(w, r, username)
		}
	})

	http.HandleFunc("/files/delete", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.Delete(w, r, username)
		}
	})

	// Запуск сервера
	log.Println("http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}