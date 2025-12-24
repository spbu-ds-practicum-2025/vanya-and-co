package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/proto"
	"google.golang.org/grpc"
)

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	grpcPort := getEnv("GRPC_PORT", "5200")
	httpPort := getEnv("HTTP_PORT", "5201") // Убедитесь что это 5201, а не 5202

	log.Printf("🚀 Starting File Service...")
	log.Printf("📁 Storage path: %s", os.Getenv("STORAGE_PATH"))
	log.Printf("💾 Database path: %s", os.Getenv("DB_PATH"))

	// Создаем сервис
	fileService := file.NewFileService()
	grpcService := file.NewGRPCService(fileService)

	// Запускаем HTTP health endpoint в отдельной горутине
	go func() {
		// Создаем новый HTTP mux для health check
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			log.Printf("Health check OK from %s", r.RemoteAddr)
		})

		log.Printf("🌐 File HTTP health service starting on :%s", httpPort)
		server := &http.Server{
			Addr:         ":" + httpPort,
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Ждем немного, чтобы HTTP сервер успел запуститься
	time.Sleep(500 * time.Millisecond)

	// Создаем gRPC сервер
	server := grpc.NewServer()
	filepb.RegisterFileServiceServer(server, grpcService)

	// Запускаем gRPC сервер
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", grpcPort, err)
	}

	log.Printf("📁 File gRPC service starting on :%s", grpcPort)
	log.Printf("✅ File service fully started (gRPC: :%s, HTTP: :%s)", grpcPort, httpPort)

	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
