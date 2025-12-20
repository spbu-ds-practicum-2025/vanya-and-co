package main

import (
	"log"
	"net"
	"os"
	"path/filepath"

	authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	"google.golang.org/grpc"
)

func main() {
	cwd, _ := os.Getwd()
	dbPath := filepath.Join(cwd, "../../data/auth.db")

	authService := authpkg.New(dbPath)

	grpcServer := grpc.NewServer()
	authpb.RegisterAuthServer(grpcServer, authService)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Println("🔐 Auth gRPC service starting on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
