package gateway

import (
	"log"
	"net/http"
)

func main() {
	// Раздаем статические файлы
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Редирект с корня на index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "./static/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	log.Println("Gateway server starting on :8000")
	log.Fatal(http.ListenAndServe(":8000", nil))
}
