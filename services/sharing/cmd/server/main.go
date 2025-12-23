package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	sharingpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing"
	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
	"google.golang.org/grpc"
)

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	grpcPort := getEnv("GRPC_PORT", "5300")

	cwd, _ := os.Getwd()
	basePath := filepath.Join(cwd, "../file/data")

	cluster := filepkg.NewCluster(basePath, 3)
	sharingService := sharingpkg.New(cluster, 7*24*time.Hour)

	// Используйте обертку для gRPC
	grpcService := sharingpkg.NewGRPCService(sharingService)

	grpcServer := grpc.NewServer()
	sharingpb.RegisterSharingServiceServer(grpcServer, grpcService)

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Запускаем HTTP сервер для sharing endpoints
	httpService := sharingService
	http.HandleFunc("/create", httpService.Create)
	http.HandleFunc("/share/", httpService.Download)
	http.HandleFunc("/list", httpService.List)
	http.HandleFunc("/get", httpService.Get)
	http.HandleFunc("/revoke", httpService.Revoke)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	go func() {
		httpPort := getEnv("HTTP_PORT", "5400")
		log.Printf("🌐 Sharing HTTP service starting on :%s", httpPort)
		if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
			log.Printf("HTTP server failed: %v", err)
		}
	}()

	log.Printf("🔗 Sharing gRPC service starting on :%s", grpcPort)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
