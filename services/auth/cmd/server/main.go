package main

import (
    "log"
    "net/http"
    "os"

    authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
)

func main() {
    // Получаем порт из переменных окружения
    httpPort := getEnv("HTTP_PORT", "5100")

    // Создаем сервис
    authService := authpkg.New("")

    // Запускаем HTTP сервер
    mux := http.NewServeMux()
    mux.HandleFunc("/auth/register", authService.Register)
    mux.HandleFunc("/auth/login", authService.LoginHandler)
    mux.HandleFunc("/auth/logout", authService.Logout)
    mux.HandleFunc("/auth/whoami", authService.WhoAmIHandler)
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    })

    log.Printf("🔐 Auth HTTP server starting on :%s", httpPort)
    log.Fatal(http.ListenAndServe(":"+httpPort, mux))
}

func getEnv(key, defaultValue string) string {
    value := os.Getenv(key)
    if value == "" {
        return defaultValue
    }
    return value
}
