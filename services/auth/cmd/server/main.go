package main

import (
    "encoding/json"
    "net/http"
    "log"
    "time"
)

// Обработчик логина
func loginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Простая логика (замените на свою)
    response := map[string]interface{}{
        "token":   "auth-token-" + time.Now().Format("20060102150405"),
        "user":    "vanya",
        "expires": time.Now().Add(24 * time.Hour).Unix(),
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Обработчик регистрации
func registerHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    response := map[string]string{
        "message": "User registered successfully",
        "status":  "active",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "service": "auth",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}

func main() {
    // Serve standalone HTML
    http.HandleFunc("/standalone", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "./static/standalone.html")
    })
    
    // API endpoints
    http.HandleFunc("/api/v1/auth/login", loginHandler)
    http.HandleFunc("/api/v1/auth/register", registerHandler)
    
    // Health check (для gateway)
    http.HandleFunc("/health", healthHandler)
    
    // Статические файлы
    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))
    
    log.Println("🔐 Auth service starting on :8001")
    log.Println("  • Standalone: http://localhost:8001/standalone")
    log.Println("  • Health: http://localhost:8001/health")
    log.Println("  • API: http://localhost:8001/api/v1/auth/login")
    
    log.Fatal(http.ListenAndServe(":8001", nil))
}
