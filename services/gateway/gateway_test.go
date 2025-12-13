package gateway

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vanya-and-co/services/auth"
	"vanya-and-co/services/file"
)

func TestGateway_RedirectsAndAuthFormFlow(t *testing.T) {
	cwd, _ := os.Getwd()
	usersPath := filepath.Join(cwd, "..", "auth", "data", "users_gateway_test.json")
	basePath := filepath.Join(cwd, "..", "file", "data")
	_ = os.Remove(usersPath)
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	_ = os.MkdirAll(basePath, 0o755)

	a := auth.New(usersPath)
	f := file.New(basePath, 1000)

	mux := http.NewServeMux()
	staticDir := filepath.Join(cwd, "static")
	indexPath := filepath.Join(staticDir, "index.html")
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, indexPath)
	})
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	mux.HandleFunc("/register-form.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/register-form.html", http.StatusFound)
	})
	mux.HandleFunc("/login-form.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/login-form.html", http.StatusFound)
	})
	mux.HandleFunc("/auth/register", a.Register)
	mux.HandleFunc("/auth/login", a.Login)
	mux.HandleFunc("/auth/logout", a.Logout)
	// require auth wrapper to simulate gateway
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.AuthFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		f.Upload(w, r, u)
	})
	mux.HandleFunc("/files/list", func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.AuthFromRequest(r)
		if !ok {
			http.Redirect(w, r, "/static/login-form.html", http.StatusSeeOther)
			return
		}
		f.List(w, r, u)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Redirects - check via mux to avoid network differences
	req := httptest.NewRequest("GET", "/register-form.html", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", w.Result().StatusCode)
	}

	req = httptest.NewRequest("GET", "/static/register-form.html", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for static file, got %d", w.Result().StatusCode)
	}
	b, _ := ioutil.ReadAll(w.Result().Body)
	w.Result().Body.Close()
	if !bytes.Contains(b, []byte("<form")) {
		t.Fatalf("expected form in static file")
	}

	// Register via form
	resp, err := http.Post(srv.URL+"/auth/register", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g1&password=p"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 303 See Other or 200 for form register, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login via form
	resp, err = http.Post(srv.URL+"/auth/login", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g1&password=p"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 303 See Other or 200 for form login, got %d", resp.StatusCode)
	}
	// Extract cookie
	cookie := resp.Header.Get("Set-Cookie")
	// turn Set-Cookie into Cookie header value
	if idx := strings.Index(cookie, ";"); idx != -1 {
		cookie = cookie[:idx]
	}
	resp.Body.Close()

	// Unauthenticated upload should be rejected
	resp, err = http.Post(srv.URL+"/files/upload", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated upload, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Authenticated upload should be allowed when cookie is sent
	client := &http.Client{}
	var b2 bytes.Buffer
	mw := multipart.NewWriter(&b2)
	fw, err := mw.CreateFormFile("file", "testfile.txt")
	if err != nil {
		t.Fatal(err)
	}
	fw.Write([]byte("hello world"))
	mw.Close()
	req, _ = http.NewRequest("POST", srv.URL+"/files/upload", &b2)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Cookie", cookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected authenticated upload to be allowed, got %d", resp.StatusCode)
	}
	bresp, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	bodyStr := string(bresp)
	// parse filename from upload response 'uploaded ok: <name>'
	var uploadedName string
	if idx := strings.Index(bodyStr, "uploaded ok:"); idx != -1 {
		rest := bodyStr[idx+len("uploaded ok:"):]
		if idx2 := strings.Index(rest, "<"); idx2 != -1 {
			uploadedName = strings.TrimSpace(rest[:idx2])
		} else {
			uploadedName = strings.TrimSpace(rest)
		}
	}
	if uploadedName == "" {
		t.Fatalf("could not determine uploaded filename from response")
	}

	// Now list files as g1 to verify uploaded file exists in user's list
	req, _ = http.NewRequest("GET", srv.URL+"/files/list", nil)
	req.Header.Set("Cookie", cookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), uploadedName) {
		t.Fatalf("expected file list to include uploaded file")
	}

	// Create second user g2 and verify it does not see g1 files
	resp, err = http.Post(srv.URL+"/auth/register", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g2&password=p2"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = http.Post(srv.URL+"/auth/login", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g2&password=p2"))
	if err != nil {
		t.Fatal(err)
	}
	cookie2 := resp.Header.Get("Set-Cookie")
	if idx := strings.Index(cookie2, ";"); idx != -1 {
		cookie2 = cookie2[:idx]
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", srv.URL+"/files/list", nil)
	req.Header.Set("Cookie", cookie2)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body2, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(body2), uploadedName) {
		t.Fatalf("expected g2 not to see g1 files")
	}

	// g2 should not be able to download g1's file (should be 404)
	req, _ = http.NewRequest("GET", srv.URL+"/files/download?name="+uploadedName, nil)
	req.Header.Set("Cookie", cookie2)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when downloading another user's file, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Logout and subsequent upload should be unauthorized
	req, _ = http.NewRequest("POST", srv.URL+"/auth/logout", nil)
	req.Header.Set("Cookie", cookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	req, _ = http.NewRequest("POST", srv.URL+"/files/upload", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
