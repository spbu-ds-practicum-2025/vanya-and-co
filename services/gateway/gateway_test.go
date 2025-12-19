package main

import (
	"bytes"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	authpkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth"
	filepkg "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file"
)

func TestGateway_RedirectsAndAuthFormFlow(t *testing.T) {
	cwd, _ := os.Getwd()
	usersPath := filepath.Join(cwd, "..", "auth", "data", "users_gateway_test.json")
	basePath := filepath.Join(cwd, "..", "file", "data")
	_ = os.Remove(usersPath)
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	_ = os.MkdirAll(basePath, 0o755)

	a := authpkg.New(usersPath)
	f := filepkg.New(basePath, 1000)

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
	mux.HandleFunc("/files/download", func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.AuthFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		// only allow owner to download in this test harness
		f.Download(w, r, u)
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

	// Use a client with cookie jar so Set-Cookie is preserved across requests
	jarAll, _ := cookiejar.New(nil)
	clientAll := &http.Client{Jar: jarAll}

	// Register via form
	resp, err := clientAll.Post(srv.URL+"/auth/register", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g1&password=p"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 303 See Other or 200 for form register, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Login via form
	resp, err = clientAll.Post(srv.URL+"/auth/login", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g1&password=p"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 303 See Other or 200 for form login, got %d", resp.StatusCode)
	}
	// Extract cookie from jar
	uurl2, _ := url.Parse(srv.URL)
	ck := jarAll.Cookies(uurl2)
	var cookie string
	for _, c := range ck {
		if c.Name == "session" {
			cookie = "session=" + c.Value
			break
		}
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

	// Quick check: ensure AuthFromRequest accepts the cookie
	reqTest, _ := http.NewRequest("POST", srv.URL+"/files/upload", nil)
	reqTest.AddCookie(&http.Cookie{Name: "session", Value: strings.TrimPrefix(cookie, "session=")})
	if u, ok := a.AuthFromRequest(reqTest); !ok || u != "g1" {
		t.Fatalf("AuthFromRequest failed: u=%q ok=%v cookie=%q", u, ok, cookie)
	}

	// Authenticated upload should be allowed when cookie is sent
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	uurl, _ := url.Parse(srv.URL)
	jar.SetCookies(uurl, []*http.Cookie{{Name: "session", Value: strings.TrimPrefix(cookie, "session=")}})
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
	// The upload handler redirects to /files/list; find the uploaded filename which contains 'testfile.txt'
	re := regexp.MustCompile(`([0-9a-fA-F]+-testfile\.txt)`)
	m := re.FindStringSubmatch(bodyStr)
	if len(m) < 2 {
		t.Fatalf("could not determine uploaded filename from response")
	}
	uploadedName := m[1]

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
	resp, err = clientAll.Post(srv.URL+"/auth/register", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g2&password=p2"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp, err = clientAll.Post(srv.URL+"/auth/login", "application/x-www-form-urlencoded", bytes.NewBufferString("username=g2&password=p2"))
	if err != nil {
		t.Fatal(err)
	}
	// Extract cookie for g2 from jar
	ck2 := jarAll.Cookies(uurl)
	var cookie2 string
	for _, c := range ck2 {
		if c.Name == "session" {
			cookie2 = "session=" + c.Value
			break
		}
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
