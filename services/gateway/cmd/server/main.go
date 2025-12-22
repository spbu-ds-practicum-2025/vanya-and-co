package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	fileClient    filepb.FileServiceClient
	sharingClient sharingpb.SharingServiceClient
	authProxy     *httputil.ReverseProxy
}

func NewGateway() (*Gateway, error) {
	// Используем адреса из переменных окружения или localhost для разработки
	authHTTPAddr := getEnv("AUTH_HTTP_ADDR", "localhost:5100")
	fileAddr := getEnv("FILE_ADDR", "localhost:5200")
	sharingAddr := getEnv("SHARE_ADDR", "localhost:5300")

	log.Printf("Connecting to Auth service at: %s (HTTP)", authHTTPAddr)
	log.Printf("Connecting to File service at: %s", fileAddr)
	log.Printf("Connecting to Sharing service at: %s", sharingAddr)

	// Подключаемся к сервисам с retry логикой
	fileConn, err := dialWithRetry(fileAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to File service: %v", err)
	}

	sharingConn, err := dialWithRetry(sharingAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to Sharing service: %v", err)
	}

	var fileClient filepb.FileServiceClient
	var sharingClient sharingpb.SharingServiceClient

	if fileConn != nil {
		fileClient = filepb.NewFileServiceClient(fileConn)
	}
	if sharingConn != nil {
		sharingClient = sharingpb.NewSharingServiceClient(sharingConn)
	}

	// Создаем reverse proxy для Auth HTTP endpoints
	authHTTPURL, _ := url.Parse("http://" + authHTTPAddr)
	authProxy := httputil.NewSingleHostReverseProxy(authHTTPURL)

	// Добавляем логирование для прокси
	originalDirector := authProxy.Director
	authProxy.Director = func(req *http.Request) {
		originalDirector(req)
		log.Printf("[Proxy] Forwarding %s %s to %s", req.Method, req.URL.Path, authHTTPURL.String())
	}
	authProxy.ModifyResponse = func(resp *http.Response) error {
		log.Printf("[Proxy] Response from Auth: Status=%s", resp.Status)
		if loc := resp.Header.Get("Location"); loc != "" {
			log.Printf("[Proxy] Redirect Location: %s", loc)
		}
		if cookies := resp.Cookies(); len(cookies) > 0 {
			for _, c := range cookies {
				log.Printf("[Proxy] Set-Cookie: %s", c.Name)
			}
		}
		return nil
	}

	return &Gateway{
		fileClient:    fileClient,
		sharingClient: sharingClient,
		authProxy:     authProxy,
	}, nil
}

func dialWithRetry(addr string, maxRetries int) (*grpc.ClientConn, error) {
	var conn *grpc.ClientConn
	var err error

	for i := 0; i < maxRetries; i++ {
		conn, err = grpc.Dial(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(2*time.Second))
		if err == nil {
			return conn, nil
		}
		log.Printf("Attempt %d/%d: Failed to connect to %s: %v", i+1, maxRetries, addr, err)
		time.Sleep(1 * time.Second)
	}
	return nil, err
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func (g *Gateway) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			log.Printf("No session cookie: %v", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if cookie.Value == "" {
			log.Printf("Empty session token")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Используем HTTP API вместо gRPC для проверки токена
		authAddr := getEnv("AUTH_HTTP_ADDR", "localhost:5100")
		authURL := "http://" + authAddr + "/auth/whoami"

		resp, err := http.PostForm(authURL, url.Values{"token": {cookie.Value}})
		if err != nil {
			log.Printf("WhoAmI HTTP request failed: %v", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("WhoAmI returned status: %d", resp.StatusCode)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		var whoamiResp struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&whoamiResp); err != nil {
			log.Printf("Failed to decode WhoAmI response: %v", err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if whoamiResp.Username == "" {
			log.Printf("WhoAmI returned empty username")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		log.Printf("Authenticated user: %s", whoamiResp.Username)
		ctx := context.WithValue(r.Context(), "username", whoamiResp.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (g *Gateway) handleFileList(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value("username").(string)

	if g.fileClient == nil {
		http.Error(w, "File service unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.fileClient.List(ctx, &filepb.ListFilesRequest{Username: username})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Files)
}

func (g *Gateway) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if g.fileClient == nil {
		http.Error(w, "File service unavailable", http.StatusServiceUnavailable)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10 MB limit
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	username := r.Context().Value("username").(string)
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.fileClient.Upload(ctx, &filepb.UploadRequest{
		Username: username,
		Filename: handler.Filename,
		Content:  content,
	})
	if err != nil || !resp.Success {
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded successfully"))
}

// ДОБАВЛЕНО: Обработчик для скачивания файлов
func (g *Gateway) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	if g.fileClient == nil {
		http.Error(w, "File service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Извлекаем file_id из URL: /files/download/{file_id}
	fileID := strings.TrimPrefix(r.URL.Path, "/files/download/")
	if fileID == "" {
		http.Error(w, "File ID is required", http.StatusBadRequest)
		return
	}

	username := r.Context().Value("username").(string)
	log.Printf("[Gateway] Download request: file_id=%s, user=%s", fileID, username)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := g.fileClient.Download(ctx, &filepb.DownloadRequest{
		FileId:   fileID,
		Username: username,
	})
	if err != nil {
		log.Printf("[Gateway] Download error: %v", err)
		http.Error(w, "Failed to download file", http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		log.Printf("[Gateway] Download failed: %s", resp.Message)
		http.Error(w, resp.Message, http.StatusNotFound)
		return
	}

	// Устанавливаем заголовки для скачивания файла
	w.Header().Set("Content-Disposition", "attachment; filename=\""+resp.Filename+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", string(len(resp.Content)))

	// Отправляем содержимое файла
	w.Write(resp.Content)
	log.Printf("[Gateway] File downloaded successfully: %s", fileID)
}

// ДОБАВЛЕНО: Обработчик для удаления файлов
func (g *Gateway) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if g.fileClient == nil {
		http.Error(w, "File service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Извлекаем file_id из URL: /files/delete/{file_id}
	fileID := strings.TrimPrefix(r.URL.Path, "/files/delete/")
	if fileID == "" {
		http.Error(w, "File ID is required", http.StatusBadRequest)
		return
	}

	username := r.Context().Value("username").(string)
	log.Printf("[Gateway] Delete request: file_id=%s, user=%s", fileID, username)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.fileClient.Delete(ctx, &filepb.DeleteRequest{
		FileId:   fileID,
		Username: username,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !resp.Success {
		http.Error(w, resp.Message, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
	})
	log.Printf("[Gateway] File deleted successfully: %s", fileID)
}

// ДОБАВЛЕНО: Обработчик для создания публичной ссылки на файл
func (g *Gateway) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if g.sharingClient == nil {
		http.Error(w, "Sharing service unavailable", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		FileID   string `json:"file_id"`
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	username := r.Context().Value("username").(string)
	log.Printf("[Gateway] Create share link: user=%s, file=%s", username, req.FileID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := g.sharingClient.CreateLink(ctx, &sharingpb.CreateLinkRequest{
		Owner:    username,
		Filename: req.Filename,
		TtlSeconds: 86400, // 24 часа
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"share_id":   response.Token,
		"expires_at": response.ExpiresAt,
	})
}

// ДОБАВЛЕНО: Обработчик для получения списка публичных ссылок
func (g *Gateway) handleListShareLinks(w http.ResponseWriter, r *http.Request) {
	if g.sharingClient == nil {
		http.Error(w, "Sharing service unavailable", http.StatusServiceUnavailable)
		return
	}

	username := r.Context().Value("username").(string)
	log.Printf("[Gateway] List share links: user=%s", username)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := g.sharingClient.GetSharedFiles(ctx, &sharingpb.GetSharedRequest{
		Username: username,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response.Files)
}

// ДОБАВЛЕНО: Обработчик для удаления публичной ссылки
func (g *Gateway) handleDeleteShareLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if g.sharingClient == nil {
		http.Error(w, "Sharing service unavailable", http.StatusServiceUnavailable)
		return
	}

	// Извлекаем share_id из URL: /api/sharing/delete/{share_id}
	shareID := strings.TrimPrefix(r.URL.Path, "/api/sharing/delete/")
	if shareID == "" {
		http.Error(w, "Share ID is required", http.StatusBadRequest)
		return
	}

	username := r.Context().Value("username").(string)
	log.Printf("[Gateway] Delete share link: user=%s, share_id=%s", username, shareID)

	// Пока sharing service не поддерживает Delete, возвращаем успех
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Share link deletion will be implemented soon",
	})
	log.Printf("[Gateway] Delete share link request received (not yet implemented): %s", shareID)
}

// ДОБАВЛЕНО: Обработчик для публичного скачивания по ссылке
func (g *Gateway) handlePublicDownload(w http.ResponseWriter, r *http.Request) {
	// Извлекаем token из URL: /share/{token}/download
	path := strings.TrimPrefix(r.URL.Path, "/share/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[1] != "download" {
		http.Error(w, "Invalid share URL", http.StatusBadRequest)
		return
	}

	token := parts[0]
	if token == "" {
		http.Error(w, "Share token is required", http.StatusBadRequest)
		return
	}

	log.Printf("[Gateway] Public download request: token=%s", token)

	// Пока что просто перенаправляем на file сервис или возвращаем ошибку
	// В будущем нужно интегрировать с sharing сервисом
	http.Error(w, "Public sharing not yet implemented", http.StatusNotImplemented)
}

func (g *Gateway) handleFilesPage(w http.ResponseWriter, r *http.Request) {
	log.Printf("[Gateway] Handling /files/list request from %s", r.RemoteAddr)

	// Определяем путь к статическим файлам
	staticPath := findStaticPath()
	dashboardPath := filepath.Join(staticPath, "dashboard.html")

	// Проверяем существование файла
	if _, err := os.Stat(dashboardPath); os.IsNotExist(err) {
		log.Printf("Dashboard file not found at: %s", dashboardPath)
		http.Error(w, "Dashboard not available", http.StatusNotFound)
		return
	}

	log.Printf("Serving dashboard from: %s", dashboardPath)
	http.ServeFile(w, r, dashboardPath)
}

func findStaticPath() string {
	// Возможные пути к статическим файлам
	paths := []string{
		"services/gateway/cmd/server/static", // при запуске из корневой директории
		"./static", // при запуске из services/gateway/cmd/server
		"static",
		"../../static",
		"/app/static", // для Docker
	}

	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
			log.Printf("Found static files at: %s", path)
			return path
		}
	}

	// Если ничего не найдено, возвращаем путь по умолчанию
	log.Printf("Static files not found, using default path: static")
	return "static"
}

func main() {
	gateway, err := NewGateway()
	if err != nil {
		log.Printf("Warning: Failed to create gateway with all services: %v", err)
	}

	// Определяем путь к статическим файлам
	staticPath := findStaticPath()
	log.Printf("Serving static files from: %s", staticPath)

	// Проверяем наличие ключевых файлов
	indexPath := filepath.Join(staticPath, "index.html")
	dashboardPath := filepath.Join(staticPath, "dashboard.html")
	
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		log.Printf("WARNING: index.html not found at %s", indexPath)
	} else {
		log.Printf("✓ Found index.html at %s", indexPath)
	}
	
	if _, err := os.Stat(dashboardPath); os.IsNotExist(err) {
		log.Printf("WARNING: dashboard.html not found at %s", dashboardPath)
	} else {
		log.Printf("✓ Found dashboard.html at %s", dashboardPath)
	}

	// Статические файлы
	fs := http.FileServer(http.Dir(staticPath))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Auth endpoints - проксируем на Auth HTTP сервис
	http.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		if gateway.authProxy != nil {
			log.Printf("Proxying register request to Auth service")
			gateway.authProxy.ServeHTTP(w, r)
		} else {
			log.Printf("Auth service unavailable for register")
			http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
		}
	})

	http.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if gateway.authProxy != nil {
			log.Printf("Proxying login request to Auth service")
			gateway.authProxy.ServeHTTP(w, r)
		} else {
			log.Printf("Auth service unavailable for login")
			http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
		}
	})

	http.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if gateway.authProxy != nil {
			log.Printf("Proxying logout request to Auth service")
			gateway.authProxy.ServeHTTP(w, r)
		} else {
			log.Printf("Auth service unavailable for logout")
			http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
		}
	})

	// API endpoints
	http.HandleFunc("/api/files/list", gateway.authMiddleware(gateway.handleFileList))
	http.HandleFunc("/files/upload", gateway.authMiddleware(gateway.handleFileUpload))
	
	// ДОБАВЛЕНО: Endpoints для скачивания и удаления файлов
	http.HandleFunc("/files/download/", gateway.authMiddleware(gateway.handleFileDownload))
	http.HandleFunc("/files/delete/", gateway.authMiddleware(gateway.handleFileDelete))

	// ДОБАВЛЕНО: Endpoints для sharing
	http.HandleFunc("/api/sharing/create", gateway.authMiddleware(gateway.handleCreateShareLink))
	http.HandleFunc("/api/sharing/list", gateway.authMiddleware(gateway.handleListShareLinks))
	http.HandleFunc("/api/sharing/delete/", gateway.authMiddleware(gateway.handleDeleteShareLink))
	http.HandleFunc("/share/", gateway.handlePublicDownload)

	http.HandleFunc("/files/list", gateway.authMiddleware(gateway.handleFilesPage))

	// Главная страница
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		
		indexPath := filepath.Join(staticPath, "index.html")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			log.Printf("Index file not found at: %s", indexPath)
			http.Error(w, "Index page not available", http.StatusNotFound)
			return
		}
		
		log.Printf("Serving index from: %s", indexPath)
		http.ServeFile(w, r, indexPath)
	})

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := getEnv("PORT", "8080")
	log.Printf("🌐 Gateway starting on http://localhost:%s", port)
	log.Printf("📁 Static files path: %s", staticPath)
	log.Printf("🔗 Available endpoints:")
	log.Printf("  - GET  /                    (Main page)")
	log.Printf("  - GET  /files/list          (Dashboard)")
	log.Printf("  - POST /auth/register       (Register)")
	log.Printf("  - POST /auth/login          (Login)")
	log.Printf("  - POST /auth/logout         (Logout)")
	log.Printf("  - GET  /api/files/list      (Files API)")
	log.Printf("  - POST /files/upload        (Upload)")
	log.Printf("  - GET  /files/download/:id  (Download)")
	log.Printf("  - POST /files/delete/:id    (Delete)")
	log.Printf("  - GET  /health              (Health check)")
	
	log.Fatal(http.ListenAndServe(":"+port, nil))
}