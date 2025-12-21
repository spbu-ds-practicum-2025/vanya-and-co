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
	"time"

	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	authClient    authpb.AuthClient
	fileClient    filepb.FileServiceClient
	sharingClient sharingpb.SharingServiceClient
	authProxy     *httputil.ReverseProxy
}

func NewGateway() (*Gateway, error) {
	// Используем адреса из переменных окружения или localhost для разработки
	authAddr := getEnv("AUTH_GRPC_ADDR", "localhost:5101")
	authHTTPAddr := getEnv("AUTH_HTTP_ADDR", "localhost:5100")
	fileAddr := getEnv("FILE_ADDR", "localhost:5200")
	sharingAddr := getEnv("SHARE_ADDR", "localhost:5300")

	log.Printf("Connecting to Auth service at: %s (gRPC) and %s (HTTP)", authAddr, authHTTPAddr)
	log.Printf("Connecting to File service at: %s", fileAddr)
	log.Printf("Connecting to Sharing service at: %s", sharingAddr)

	// Подключаемся к сервисам с retry логикой
	authConn, err := dialWithRetry(authAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to Auth gRPC service: %v", err)
	}

	fileConn, err := dialWithRetry(fileAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to File service: %v", err)
	}

	sharingConn, err := dialWithRetry(sharingAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to Sharing service: %v", err)
	}

	var authClient authpb.AuthClient
	var fileClient filepb.FileServiceClient
	var sharingClient sharingpb.SharingServiceClient

	if authConn != nil {
		authClient = authpb.NewAuthClient(authConn)
	}
	if fileConn != nil {
		fileClient = filepb.NewFileServiceClient(fileConn)
	}
	if sharingConn != nil {
		sharingClient = sharingpb.NewSharingServiceClient(sharingConn)
	}

	// Создаем reverse proxy для Auth HTTP endpoints
	authHTTPURL, _ := url.Parse("http://" + authHTTPAddr)
	authProxy := httputil.NewSingleHostReverseProxy(authHTTPURL)

	return &Gateway{
		authClient:    authClient,
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

		if g.authClient == nil {
			log.Printf("Auth service unavailable")
			http.Error(w, "Auth service unavailable", http.StatusServiceUnavailable)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := g.authClient.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: cookie.Value})
		if err != nil || resp.Username == "" {
			log.Printf("WhoAmI failed: Error=%v, Username=%v", err, resp.Username)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		log.Printf("Authenticated user: %s", resp.Username)
		ctx = context.WithValue(r.Context(), "username", resp.Username)
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

// ИСПРАВЛЕНО: Новый обработчик для /files/list с правильной авторизацией
func (g *Gateway) handleFilesPage(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию через middleware
	cookie, err := r.Cookie("session")
	if err != nil {
		log.Printf("No session cookie for /files/list: %v", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if g.authClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := g.authClient.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: cookie.Value})
		if err != nil || resp.Username == "" {
			log.Printf("WhoAmI failed for /files/list: Error=%v, Username=%v", err, resp.Username)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		log.Printf("User %s accessing dashboard", resp.Username)
	}

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

// ДОБАВЛЕНО: Функция для поиска статических файлов
func findStaticPath() string {
	// Возможные пути к статическим файлам
	paths := []string{
		"static",
		"./static",
		"services/gateway/cmd/server/static",
		"./services/gateway/cmd/server/static",
		"../../static",
		"/app/static", // для Docker
	}

	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "index.html")); err == nil {
			return path
		}
	}

	// Если ничего не найдено, возвращаем первый путь
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
	
	// ИСПРАВЛЕНО: /files/list теперь использует улучшенный обработчик
	http.HandleFunc("/files/list", gateway.handleFilesPage)

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
	log.Printf("  - GET  /health              (Health check)")
	
	log.Fatal(http.ListenAndServe(":"+port, nil))
}