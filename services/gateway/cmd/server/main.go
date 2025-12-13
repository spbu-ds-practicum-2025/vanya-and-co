package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func whoami(r *http.Request, authURL string) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", authURL+"/auth/whoami", nil)
	for _, c := range r.Cookies() {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func proxyWithUser(target *url.URL, authURL string) http.Handler {
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		if user, _ := whoami(req, authURL); user != "" {
			req.Header.Set("X-User", user)
		}
	}
	return &httputil.ReverseProxy{Director: director}
}

func proxyPlain(target *url.URL) http.Handler {
	director := func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}
	return &httputil.ReverseProxy{Director: director}
}

func main() {
	cwd, _ := os.Getwd()
	log.Printf("Running from: %s", cwd)

	staticDir := filepath.Join(cwd, "static")
	log.Printf("Static directory: %s", staticDir)

	authAddr := getEnv("AUTH_ADDR", "http://localhost:5100")
	fileAddr := getEnv("FILE_ADDR", "http://localhost:5200")
	shareAddr := getEnv("SHARE_ADDR", "http://localhost:5300")

	authURL, _ := url.Parse(authAddr)
	fileURL, _ := url.Parse(fileAddr)
	shareURL, _ := url.Parse(shareAddr)

	// auth endpoints
	http.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		proxyPlain(authURL).ServeHTTP(w, r)
	})
	http.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		proxyPlain(authURL).ServeHTTP(w, r)
	})
	http.HandleFunc("/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		proxyPlain(authURL).ServeHTTP(w, r)
	})

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if user, _ := whoami(r, authAddr); user != "" {
			http.Redirect(w, r, "/files/list", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})

	http.Handle("/files/", proxyWithUser(fileURL, authAddr))
	http.Handle("/share/", proxyPlain(shareURL))

	http.HandleFunc("/static/share-form.html", func(w http.ResponseWriter, r *http.Request) {
		if user, _ := whoami(r, authAddr); user != "" {
			http.ServeFile(w, r, filepath.Join(staticDir, "share-form.html"))
			return
		}
		http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
	})

	http.HandleFunc("/static/my-shares.html", func(w http.ResponseWriter, r *http.Request) {
		if user, _ := whoami(r, authAddr); user != "" {
			http.ServeFile(w, r, filepath.Join(staticDir, "my-shares.html"))
			return
		}
		http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
	})

	log.Println("Gateway starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
