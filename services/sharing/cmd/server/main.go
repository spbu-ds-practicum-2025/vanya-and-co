package main

import (
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	sharingpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing"
	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
	"google.golang.org/grpc"
)

func main() {
	cwd, _ := os.Getwd()
	basePath := filepath.Join(cwd, "../file/data")

	cluster := filepkg.NewCluster(basePath, 3)
	sharingService := sharingpkg.New(cluster, 7*24*time.Hour)

	// Используйте обертку для gRPC
	grpcService := sharingpkg.NewGRPCService(sharingService)

	grpcServer := grpc.NewServer()
	sharingpb.RegisterSharingServiceServer(grpcServer, grpcService)

	lis, err := net.Listen("tcp", ":50053")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Println("🔗 Sharing gRPC service starting on :50053")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
