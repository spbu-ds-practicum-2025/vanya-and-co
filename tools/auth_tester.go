//go:build tools
// +build tools

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"path/filepath"

	"vanya-and-co/services/auth"
)

func main() {
	cwd, _ := os.Getwd()
	usersPath := filepath.Join(cwd, "services", "auth", "data", "users.json")
	// remove the file to start fresh
	_ = os.Remove(usersPath)
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)

	svc := auth.New(store)

	// register a test user
	body := bytes.NewBufferString(`{"username":"testuser","password":"123"}`)
	req := httptest.NewRequest("POST", "/auth/register", body)
	w := httptest.NewRecorder()
	svc.Register(w, req)
	resp := w.Result()
	fmt.Println("register status:", resp.StatusCode)

	// print users.json
	data, _ := ioutil.ReadFile(usersPath)
	fmt.Println("users.json content:")
	fmt.Println(string(data))

	// login
	body = bytes.NewBufferString(`{"username":"testuser","password":"123"}`)
	req = httptest.NewRequest("POST", "/auth/login", body)
	w = httptest.NewRecorder()
	svc.Login(w, req)
	resp = w.Result()
	fmt.Println("login status:", resp.StatusCode)
}
