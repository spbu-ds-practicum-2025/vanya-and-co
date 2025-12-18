package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	"google.golang.org/grpc"
)

func main() {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(cwd))) // move up to repo root when running from cmd/server
	usersPath := filepath.Join(projectRoot, "services", "auth", "data", "users.json")
	svc := authpkg.New(usersPath)

	// endpoints
	http.HandleFunc("/auth/login", svc.Login)
	http.HandleFunc("/auth/register", svc.Register)
	http.HandleFunc("/auth/logout", svc.Logout)
	http.HandleFunc("/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
		if user, ok := svc.AuthFromRequest(r); ok {
			w.Write([]byte(user))
			return
		}
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	// Start gRPC server for Auth service
	go func() {
		lis, err := net.Listen("tcp", ":5101")
		if err != nil {
			log.Printf("grpc listen error: %v", err)
			return
		}
		grpcSrv := grpc.NewServer()
		authpb.RegisterAuthServer(grpcSrv, svc)
		log.Printf("auth gRPC listening on %s", ":5101")
		grpcSrv.Serve(lis)
	}()

	addr := ":5100"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("auth service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
