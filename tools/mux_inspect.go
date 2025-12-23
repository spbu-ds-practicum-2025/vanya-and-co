package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/register-form.html", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/login-form.html", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/upload-form.html", func(w http.ResponseWriter, r *http.Request) {})
	// create a test request
	req := httptest.NewRequest("GET", "/upload-form.html", nil)
	h, pattern := mux.Handler(req)
	fmt.Printf("pattern matched: %q\n", pattern)
	fmt.Printf("handler address: %v\n", h)
}
