package main

import (
	"log"
	"net"
	"os"

	"github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	"google.golang.org/grpc"
)

func main() {
	// Получаем порт из переменной окружения или используем значение по умолчанию
	grpcPort := getEnv("GRPC_PORT", "5200")

	// Создаем сервис
	fileService := file.NewFileService()
	grpcService := file.NewGRPCService(fileService)

	// Создаем gRPC сервер
	server := grpc.NewServer()
	filepb.RegisterFileServiceServer(server, grpcService)

	// Запускаем сервер
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("📁 File service starting on :%s", grpcPort)
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
