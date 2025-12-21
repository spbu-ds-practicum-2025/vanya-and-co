package auth

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setup(t *testing.T) string {
	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, "data", "auth_test.db")
	_ = os.Remove(p)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return p
}

func TestRegisterAndLogin_JSON(t *testing.T) {
	p := setup(t)
	s := New(p)

	body := map[string]string{"username": "u1", "password": "p"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Register(w, req)
	if w.Result().StatusCode != http.StatusCreated && w.Result().StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 201 or 303 got %d", w.Result().StatusCode)
	}

	// login - ИСПРАВЛЕНО: используем LoginHandler вместо Login
	req = httptest.NewRequest("POST", "/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	s.LoginHandler(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Result().StatusCode)
	}

	// Check DB has user
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	var u string
	if err := db.QueryRow(`SELECT username FROM users WHERE username = ?`, "u1").Scan(&u); err != nil || u != "u1" {
		t.Fatalf("user not found in db: %v", err)
	}
	db.Close()
}

func TestRegisterAndLogin_Form(t *testing.T) {
	p := setup(t)
	s := New(p)

	form := "username=u2&password=pp"
	req := httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Register(w, req)
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303 got %d", w.Result().StatusCode)
	}

	// ИСПРАВЛЕНО: используем LoginHandler вместо Login
	req = httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	s.LoginHandler(w, req)
	if w.Result().StatusCode != http.StatusSeeOther {
		t.Fatalf("expected 303 got %d", w.Result().StatusCode)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	p := setup(t)
	s := New(p)

	form := "username=u3&password=ppp"
	req := httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Register(w, req)

	// ИСПРАВЛЕНО: используем LoginHandler вместо Login
	req = httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w = httptest.NewRecorder()
	s.LoginHandler(w, req)
	if w.Result().StatusCode != http.StatusSeeOther && w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected login redirect/ok got %d", w.Result().StatusCode)
	}
	cookie := w.Result().Header.Get("Set-Cookie")
	if cookie == "" {
		t.Fatalf("expected Set-Cookie header")
	}
	// convert Set-Cookie to Cookie header value
	if idx := bytes.IndexByte([]byte(cookie), ';'); idx != -1 {
		cookie = cookie[:idx]
	}

	// AuthFromRequest should succeed when cookie is present
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Cookie", cookie)
	if _, ok := s.AuthFromRequest(r); !ok {
		t.Fatalf("expected AuthFromRequest to return ok for logged in user")
	}

	// Call logout, it should remove session
	req = httptest.NewRequest("POST", "/auth/logout", nil)
	req.Header.Set("Cookie", cookie)
	w = httptest.NewRecorder()
	s.Logout(w, req)

	// AuthFromRequest should now return false
	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Cookie", cookie)
	if _, ok := s.AuthFromRequest(r); ok {
		t.Fatalf("expected AuthFromRequest to fail after logout")
	}
}