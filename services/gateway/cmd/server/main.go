package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
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
}

func NewGateway() (*Gateway, error) {
	// Используем адреса из переменных окружения или localhost для разработки
	authAddr := getEnv("AUTH_GRPC_ADDR", "localhost:5101")
	fileAddr := getEnv("FILE_ADDR", "localhost:5200")
	sharingAddr := getEnv("SHARE_ADDR", "localhost:5300")

	log.Printf("Connecting to Auth service at: %s", authAddr)
	log.Printf("Connecting to File service at: %s", fileAddr)
	log.Printf("Connecting to Sharing service at: %s", sharingAddr)

	// Подключаемся к сервисам с retry логикой
	authConn, err := dialWithRetry(authAddr, 5)
	if err != nil {
		log.Printf("Warning: Failed to connect to Auth service: %v", err)
		// Продолжаем работу даже если Auth недоступен
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

	return &Gateway{
		authClient:    authClient,
		fileClient:    fileClient,
		sharingClient: sharingClient,
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

func (g *Gateway) handleFilesPage(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if g.authClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := g.authClient.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: cookie.Value})
		if err != nil || resp.Username == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	// Отдаем страницу dashboard
	http.ServeFile(w, r, filepath.Join("static", "dashboard.html"))
}

func main() {
	gateway, err := NewGateway()
	if err != nil {
		log.Printf("Warning: Failed to create gateway with all services: %v", err)
	}

	// Определяем путь к статическим файлам
	staticPath := "static"
	if _, err := os.Stat("static"); os.IsNotExist(err) {
		// Пробуем альтернативный путь
		staticPath = "../../static"
		if _, err := os.Stat(staticPath); os.IsNotExist(err) {
			staticPath = "./services/gateway/cmd/server/static"
		}
	}
	log.Printf("Serving static files from: %s", staticPath)

	// Статические файлы
	fs := http.FileServer(http.Dir(staticPath))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// API endpoints
	http.HandleFunc("/api/files/list", gateway.authMiddleware(gateway.handleFileList))
	http.HandleFunc("/files/upload", gateway.authMiddleware(gateway.handleFileUpload))
	http.HandleFunc("/files/list", gateway.handleFilesPage)

	// Главная страница
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticPath, "index.html"))
	})

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := getEnv("PORT", "8080")
	log.Printf("🌐 Gateway starting on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
