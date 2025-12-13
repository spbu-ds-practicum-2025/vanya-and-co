package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"vanya-and-co/services/auth"
	"vanya-and-co/services/file"
	"vanya-and-co/services/sharing"
)

func main() {
	cwd, _ := os.Getwd()
	log.Printf("Running from: %s", cwd)

	// 1. Инициализация сервисов
	projectRoot := filepath.Dir(cwd) // services/
	projectRoot = filepath.Dir(projectRoot) // корень (vanya-and-co/)
	
	authPath := filepath.Join(projectRoot, "services", "auth", "data", "users.json")
	authService := auth.New(authPath)
	
	baseFileStorage := filepath.Join(projectRoot, "services", "file", "data")
	fileService := file.New(baseFileStorage, 10)

	// Создаем сервис шаринга (TTL 7 дней)
	cluster := file.NewCluster(baseFileStorage, 3)
	shareService := sharing.New(cluster, 7*24*time.Hour)

	// Создаем директории если их нет
	os.MkdirAll(filepath.Dir(authPath), 0755)
	os.MkdirAll(baseFileStorage, 0755)

	// Статическая директория
	staticDir := filepath.Join(cwd, "static")
	log.Printf("Static directory: %s", staticDir)

	// Вспомогательные функции
	requireAuth := func(w http.ResponseWriter, r *http.Request) (username string, ok bool) {
		username, ok = authService.AuthFromRequest(r)
		if !ok {
			http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
		}
		return username, ok
	}

	getUsername := func(r *http.Request) string {
		username, _ := authService.AuthFromRequest(r)
		return username
	}
	
	// 2. Обработчики Авторизации
	http.HandleFunc("/auth/login", authService.Login)
	http.HandleFunc("/auth/register", authService.Register)
	http.HandleFunc("/auth/logout", authService.Logout)

	// 3. Обработчик Статики
	http.Handle("/static/", http.StripPrefix("/static/", 
		http.FileServer(http.Dir(staticDir))))

	// 4. Главная страница
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		
		if _, ok := authService.AuthFromRequest(r); ok {
			http.Redirect(w, r, "/files/list", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	// 5. Обработчики Файлов
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
			// Проверка доступа через шаринг
			requestedUser := r.URL.Query().Get("user")
			if requestedUser != "" && requestedUser != username {
				// Проверяем есть ли публичная ссылка
				filename := r.URL.Query().Get("name")
				relPath := requestedUser + "/" + filename
				
				// Здесь можно добавить проверку через shareService
				// Пока просто разрешаем если user указан
				fileService.Download(w, r, requestedUser)
				return
			}
			fileService.Download(w, r, username)
		}
	})

	http.HandleFunc("/files/delete", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.Delete(w, r, username)
		}
	})

	// 6. Обработчики Шаринга
	http.HandleFunc("/share/create", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			// Автоматически добавляем owner если не указан
			if r.URL.Query().Get("owner") == "" {
				// Формируем новый URL с owner
				fileParam := r.URL.Query().Get("file")
				if fileParam != "" && !strings.Contains(fileParam, "/") {
					fileParam = username + "/" + fileParam
				}
				
				// Создаем новый запрос с параметром owner
				query := r.URL.Query()
				query.Set("owner", username)
				query.Set("file", fileParam)
				r.URL.RawQuery = query.Encode()
			}
			shareService.Create(w, r)
		}
	})

	http.HandleFunc("/share/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		
		switch {
		case strings.HasSuffix(path, "/download"):
			// Публичное скачивание - без авторизации
			shareService.Download(w, r)
		case strings.Contains(path, "/list"):
			// Список ссылок пользователя - требует авторизации
			if username, ok := requireAuth(w, r); ok {
				query := r.URL.Query()
				if query.Get("owner") == "" {
					query.Set("owner", username)
					r.URL.RawQuery = query.Encode()
				}
				shareService.List(w, r)
			}
		default:
			// GET /share/:token или DELETE /share/:token
			if r.Method == "GET" {
				shareService.Get(w, r)
			} else if r.Method == "DELETE" {
				if _, ok := requireAuth(w, r); ok {
					shareService.Revoke(w, r)
				}
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	// 7. HTML страницы для шаринга
	http.HandleFunc("/static/share-form.html", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(w, r); ok {
			http.ServeFile(w, r, filepath.Join(staticDir, "share-form.html"))
		}
	})

	http.HandleFunc("/static/my-shares.html", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(w, r); ok {
			http.ServeFile(w, r, filepath.Join(staticDir, "my-shares.html"))
		}
	})

	// Запуск сервера
	log.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}