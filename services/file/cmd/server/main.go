// services/file/cmd/server/main.go
package main

import (
    "encoding/json"
    "net/http"
    "log"
    "time"
)

// Health check
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
        "service": "file",
        "timestamp": time.Now().Format(time.RFC3339),
    })
}

func main() {
    // Standalone HTML
    http.HandleFunc("/standalone", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "./static/standalone.html")
    })
    
    // API endpoints (заглушки)
    http.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
        files := []map[string]string{
            {"id": "1", "name": "document.pdf", "size": "2.3MB"},
            {"id": "2", "name": "image.png", "size": "1.1MB"},
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(files)
    })
    
    // Health check
    http.HandleFunc("/health", healthHandler)
    
    log.Println("📁 File service starting on :8002")
    log.Fatal(http.ListenAndServe(":8002", nil))
}
