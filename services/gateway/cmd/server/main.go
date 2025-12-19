package main

import (
    "net/http"
    "log"
    "os"
    "path/filepath"
    "strings"
)

// Функция проверки существования файла
func fileExists(filename string) bool {
    info, err := os.Stat(filename)
    if os.IsNotExist(err) {
        return false
    }
    return !info.IsDir()
}

func main() {
    port := ":8000"
    
    // Маппинг URL -> файлов
    routes := map[string]string{
        "/":               "index.html",
        "/login":          "login-form.html",
        "/login-form":     "login-form.html",
        "/login-form.html": "login-form.html",
        "/register":       "register-form.html",
        "/register-form":  "register-form.html",
        "/dashboard":      "dashboard.html",
        "/upload":         "upload-form.html",
        "/download":       "download-form.html",
        "/share":          "share-form.html",
        "/my-shares":      "my-shares.html",
    }
    
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        urlPath := r.URL.Path
        log.Printf("📄 Request: %s %s", r.Method, urlPath)
        
        // Пробуем найти в маппинге
        if filename, exists := routes[urlPath]; exists {
            filePath := filepath.Join("./static", filename)
            if fileExists(filePath) {
                log.Printf("   ✅ Serving: %s", filePath)
                http.ServeFile(w, r, filePath)
                return
            } else {
                log.Printf("   ❌ File not found: %s", filePath)
            }
        }
        
        // Если запрашивают файл напрямую с .html
        if strings.HasSuffix(urlPath, ".html") {
            filePath := filepath.Join("./static", urlPath)
            if fileExists(filePath) {
                log.Printf("   ✅ Serving direct: %s", filePath)
                http.ServeFile(w, r, filePath)
                return
            }
        }
        
        // Если это API запрос
        if strings.HasPrefix(urlPath, "/api/") {
            log.Printf("   🔌 API call: %s", urlPath)
            w.WriteHeader(http.StatusOK)
            w.Write([]byte("API endpoint"))
            return
        }
        
        // Пробуем найти файл в static
        filePath := filepath.Join("./static", urlPath)
        if fileExists(filePath) {
            log.Printf("   ✅ Serving static: %s", filePath)
            http.ServeFile(w, r, filePath)
            return
        }
        
        // Если ничего не найдено - 404
        log.Printf("   ❓ Not found, serving index.html")
        http.ServeFile(w, r, "./static/index.html")
    })

    log.Println("Vanya-and-co Gateway Server")
    log.Println("Server started on http://localhost" + port)
    log.Println("Available pages:")
    log.Println(" • Main page:      http://localhost" + port + "/")
    log.Println(" • Login:          http://localhost" + port + "/login-form")
    log.Println(" • Register:       http://localhost" + port + "/register-form")
    log.Println(" • Dashboard:      http://localhost" + port + "/dashboard")
    log.Println(" • Upload:         http://localhost" + port + "/upload-form")
    log.Println(" • Download:       http://localhost" + port + "/download-form")
    log.Println(" • Share:          http://localhost" + port + "/share-form")
    log.Println(" • My Shares:      http://localhost" + port + "/my-shares")

    if err := http.ListenAndServe(port, nil); err != nil {
        log.Fatal("Server error: ", err)
    }
}
