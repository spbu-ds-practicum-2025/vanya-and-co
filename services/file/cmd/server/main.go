package main

import (
	"log"
	"net"

	"github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	"google.golang.org/grpc"
)

func main() {
	// Создаем сервис
	fileService := file.NewFileService()
	grpcService := file.NewGRPCService(fileService)

	// Создаем gRPC сервер
	server := grpc.NewServer()
	filepb.RegisterFileServiceServer(server, grpcService)

	// Запускаем сервер
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("📁 File service starting on :50052")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
