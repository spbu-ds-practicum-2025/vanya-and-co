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
	// Текущая директория - services/gateway/
	cwd, _ := os.Getwd()
	log.Printf("Running from: %s", cwd)

	// 1. Инициализация сервисов
	// Поднимаемся на уровень выше к корню проекта
	projectRoot := filepath.Dir(cwd) // services/
	projectRoot = filepath.Dir(projectRoot) // корень (vanya-and-co/)
	
	log.Printf("Project root: %s", projectRoot)
	
	authPath := filepath.Join(projectRoot, "services", "auth", "data", "users.json")
	log.Printf("Auth path: %s", authPath)
	authService := auth.New(authPath)
	
	baseFileStorage := filepath.Join(projectRoot, "services", "file", "data")
	log.Printf("File storage: %s", baseFileStorage)
	fileService := file.New(baseFileStorage, 10)

	// Создаем директории если их нет
	os.MkdirAll(filepath.Dir(authPath), 0755)
	os.MkdirAll(baseFileStorage, 0755)

	// Статическая директория - В ТЕКУЩЕЙ ПАПКЕ (gateway/static)
	staticDir := filepath.Join(cwd, "static")
	log.Printf("Static directory: %s", staticDir)

	// Проверяем что папка существует
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		log.Fatalf("ERROR: Static directory not found at %s", staticDir)
	}

	// Проверяем что файлы есть
	files, _ := os.ReadDir(staticDir)
	log.Printf("Found %d files in static directory", len(files))
	for _, f := range files {
		log.Printf("  - %s", f.Name())
	}

	// Вспомогательная функция для проверки авторизации
	requireAuth := func(w http.ResponseWriter, r *http.Request) (username string, ok bool) {
		username, ok = authService.AuthFromRequest(r)
		if !ok {
			http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
		}
		return username, ok
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
		// Если запрос не к корню, отдаем 404
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		
		// Если пользователь авторизован, сразу кидаем на файлы
		if _, ok := authService.AuthFromRequest(r); ok {
			http.Redirect(w, r, "/files/list", http.StatusSeeOther)
			return
		}
		// Иначе показываем главную
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
			fileService.Download(w, r, username)
		}
	})

	http.HandleFunc("/files/delete", func(w http.ResponseWriter, r *http.Request) {
		if username, ok := requireAuth(w, r); ok {
			fileService.Delete(w, r, username)
		}
	})

	// Запуск сервера
	log.Println("Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}