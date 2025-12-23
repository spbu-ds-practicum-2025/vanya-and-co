package main

import (
    "log"
    "net"
    "net/http"
    "os"
    _"strconv"

    authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
    authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
    "google.golang.org/grpc"
)

func main() {
    // Получаем порты из переменных окружения
    httpPort := getEnv("HTTP_PORT", "5100")
    grpcPort := getEnv("GRPC_PORT", "5101")
    
    // Создаем сервис
    authService := authpkg.New("")

    // Запускаем HTTP сервер
    go func() {
        mux := http.NewServeMux()
        mux.HandleFunc("/auth/register", authService.Register)
        mux.HandleFunc("/auth/login", authService.LoginHandler)
        mux.HandleFunc("/auth/logout", authService.Logout)
        mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("OK"))
        })
        
        log.Printf("🔐 Auth HTTP server starting on :%s", httpPort)
        log.Fatal(http.ListenAndServe(":"+httpPort, mux))
    }()

    // Запускаем gRPC сервер
    lis, err := net.Listen("tcp", ":"+grpcPort)
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()
    authpb.RegisterAuthServer(grpcServer, authService)

    log.Printf("Auth gRPC service starting on :%s", grpcPort)
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
