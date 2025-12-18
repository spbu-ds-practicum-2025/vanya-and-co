package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
	sharepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var authGRPCClient authpb.AuthClient

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func whoami(r *http.Request, authURL string) (string, error) {
	// Optional debug
	debug := os.Getenv("GATEWAY_DEBUG") == "1"
	if debug {
		log.Printf("whoami: request %s %s from %s", r.Method, r.URL.String(), r.RemoteAddr)
	}

	// Prefer gRPC auth when available
	if authGRPCClient != nil {
		var token string
		for _, c := range r.Cookies() {
			if c.Name == "session" {
				token = c.Value
				break
			}
		}
		if token != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
			defer cancel()
			if resp, err := authGRPCClient.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: token}); err == nil {
				if debug {
					log.Printf("whoami: gRPC token=%s -> user=%q", token, resp.Username)
				}
				return resp.Username, nil
			} else {
				if debug {
					log.Printf("whoami: gRPC error: %v", err)
				}
			}
		}
	}
	// fallback to HTTP
	client := &http.Client{}
	req, _ := http.NewRequest("GET", authURL+"/auth/whoami", nil)
	for _, c := range r.Cookies() {
		req.AddCookie(c)
	}
	resp, err := client.Do(req)
	if err != nil {
		if debug {
			log.Printf("whoami: http fallback error: %v", err)
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if debug {
			log.Printf("whoami: http fallback status %d", resp.StatusCode)
		}
		return "", nil
	}
	b, _ := io.ReadAll(resp.Body)
	if debug {
		log.Printf("whoami: http fallback -> user=%q", string(b))
	}
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

	// Use gateway's local static directory
	staticDir := filepath.Join(cwd, "services", "gateway", "cmd", "server", "static")
	log.Printf("Static directory: %s", staticDir)

	authAddr := getEnv("AUTH_ADDR", "http://localhost:5100")
	fileAddr := getEnv("FILE_ADDR", "http://localhost:5200")
	shareAddr := getEnv("SHARE_ADDR", "http://localhost:5300")

	authURL, _ := url.Parse(authAddr)
	fileURL, _ := url.Parse(fileAddr)
	shareURL, _ := url.Parse(shareAddr)

	// Dial auth gRPC server (if available)
	authGRPCAddr := getEnv("AUTH_GRPC_ADDR", "localhost:5101")
	if conn, err := grpc.Dial(authGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials())); err == nil {
		authGRPCClient = authpb.NewAuthClient(conn)
		log.Printf("connected to auth gRPC at %s", authGRPCAddr)
	} else {
		log.Printf("auth gRPC dial error: %v", err)
	}

	// If services are not reachable, embed lightweight versions locally so
	// `go run ./services/gateway/cmd/server` starts a working demo.
	// Default behavior: if any target is down, start embedded services.
	authUp := isUp(authAddr + "/auth/whoami")
	fileUp := isUp(fileAddr + "/files/list")
	shareUp := isUp(shareAddr + "/share/list")
	if !(authUp && fileUp && shareUp) {
		log.Println("Some backend services are down; starting embedded services...")
		go startEmbeddedServices()
		// give embedded servers a moment to start
		time.Sleep(200 * time.Millisecond)
	}

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
		// try serve static index.html if present, otherwise show built-in homepage
		idx := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(idx); err == nil {
			http.ServeFile(w, r, idx)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<!doctype html>
<html><head><meta charset="utf-8"><title>vanya-and-co Gateway</title></head><body>
<h1>vanya-and-co Gateway</h1>
<p>This is a development gateway. Use the links below to explore services:</p>
<ul>
  <li><a href="/auth/register?user=alice&pass=secret">Register (example)</a></li>
  <li><a href="/auth/login?user=alice&pass=secret">Login (example)</a></li>
  <li><a href="/files/list">Files (requires login)</a></li>
  <li><a href="/static/login-form.html">Login form (if present)</a></li>
  <li><a href="/static/share-form.html">Share form (if present)</a></li>
  <li><a href="/static/my-shares.html">My shares (if present)</a></li>
</ul>
</body></html>`)
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

	port := getEnv("PORT", "8080")
	fmt.Printf("Gateway starting on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func isUp(u string) bool {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, err := client.Get(u)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// startEmbeddedServices runs minimal auth/file/sharing HTTP servers on the
// default ports. Useful for local dev when services aren't running separately.
func startEmbeddedServices() {
	cwd, _ := os.Getwd()

	// Auth
	go func() {
		usersPath := filepath.Join(cwd, "services", "auth", "data", "users_embedded.json")
		a := authpkg.New(usersPath)
		mux := http.NewServeMux()
		mux.HandleFunc("/auth/login", a.Login)
		mux.HandleFunc("/auth/register", a.Register)
		mux.HandleFunc("/auth/logout", a.Logout)
		mux.HandleFunc("/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
			if u, ok := a.AuthFromRequest(r); ok {
				w.Write([]byte(u))
				return
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		})
		addr := ":5100"
		log.Printf("embedded auth listening on %s", addr)
		http.ListenAndServe(addr, mux)
	}()

	// File
	go func() {
		base := filepath.Join(cwd, "services", "file", "data")
		f := filepkg.New(base, 10)
		mux := http.NewServeMux()
		requireUser := func(w http.ResponseWriter, r *http.Request) (string, bool) {
			u := r.Header.Get("X-User")
			if u == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return "", false
			}
			return u, true
		}
		mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
			if u, ok := requireUser(w, r); ok {
				f.Upload(w, r, u)
			}
		})
		mux.HandleFunc("/files/list", func(w http.ResponseWriter, r *http.Request) {
			if u, ok := requireUser(w, r); ok {
				f.List(w, r, u)
			}
		})
		mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
			if u, ok := requireUser(w, r); ok {
				if other := r.URL.Query().Get("user"); other != "" && other != u {
					f.Download(w, r, other)
					return
				}
				f.Download(w, r, u)
			}
		})
		mux.HandleFunc("/files/delete", func(w http.ResponseWriter, r *http.Request) {
			if u, ok := requireUser(w, r); ok {
				f.Delete(w, r, u)
			}
		})
		addr := ":5200"
		log.Printf("embedded file listening on %s", addr)
		http.ListenAndServe(addr, mux)
	}()

	// Sharing
	go func() {
		cluster := filepkg.NewCluster(filepath.Join(cwd, "services", "file", "data", "cluster_embedded"), 3)
		s := sharepkg.New(cluster, 7*24*time.Hour)
		mux := http.NewServeMux()
		mux.HandleFunc("/share/create", s.Create)
		mux.HandleFunc("/share/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			switch {
			case r.Method == "GET" && (len(path) > 7 && strings.HasSuffix(path, "/download")):
				s.Download(w, r)
			case r.Method == "GET" && (path == "/share/" || path == "/share/list"):
				s.List(w, r)
			case r.Method == "GET":
				s.Get(w, r)
			case r.Method == "DELETE":
				s.Revoke(w, r)
			default:
				http.Error(w, "not found", http.StatusNotFound)
			}
		})
		addr := ":5300"
		log.Printf("embedded sharing listening on %s", addr)
		http.ListenAndServe(addr, mux)
	}()
}
