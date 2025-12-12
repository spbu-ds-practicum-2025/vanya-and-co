package auth

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthHandlers_Integration(t *testing.T) {
	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, "data", "users_integ.json")
	_ = os.Remove(p)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	s := New(p)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", s.Register)
	mux.HandleFunc("/auth/login", s.Login)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// register via JSON
	body := bytes.NewBufferString(`{"username":"iuser","password":"pw"}`)
	resp, err := http.Post(srv.URL+"/auth/register", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("reg status %d", resp.StatusCode)
	}
	resp.Body.Close()

	// login
	body = bytes.NewBufferString(`{"username":"iuser","password":"pw"}`)
	resp, err = http.Post(srv.URL+"/auth/login", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", resp.StatusCode)
	}
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "OK" {
		t.Fatalf("unexpected body %s", string(b))
	}

	// verify file exists
	data, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("users file len", len(data))

	// redirect behavior tested in gateway package
}
