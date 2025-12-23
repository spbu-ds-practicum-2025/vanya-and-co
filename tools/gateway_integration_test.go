//go:build tools
// +build tools

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"vanya-and-co/services/auth"
)

func main() {
	cwd, _ := os.Getwd()
	usersPath := filepath.Join(cwd, "services", "auth", "data", "users.json")
	basePath := filepath.Join(cwd, "services", "file", "data")
	_ = os.Remove(usersPath)
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	_ = os.MkdirAll(basePath, 0o755)

	a := auth.New(usersPath)
	f := file.New(basePath, 1000)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/register", a.Register)
	mux.HandleFunc("/auth/login", a.Login)
	mux.HandleFunc("/files/upload", func(w http.ResponseWriter, r *http.Request) {
		u, ok := a.AuthFromRequest(r)
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		f.Upload(w, r, u)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Register
	registerBody := bytes.NewBufferString(`{"username":"integ","password":"p4ssw0rd"}`)
	resp, err := http.Post(srv.URL+"/auth/register", "application/json", registerBody)
	if err != nil {
		panic(err)
	}
	fmt.Println("register status:", resp.StatusCode)
	_ = resp.Body.Close()

	// Read users file
	data, _ := ioutil.ReadFile(usersPath)
	fmt.Println("users.json content:")
	fmt.Println(string(data))

	// Login
	loginBody := bytes.NewBufferString(`{"username":"integ","password":"p4ssw0rd"}`)
	resp, err = http.Post(srv.URL+"/auth/login", "application/json", loginBody)
	if err != nil {
		panic(err)
	}
	fmt.Println("login status:", resp.StatusCode)
	b, _ := ioutil.ReadAll(resp.Body)
	fmt.Println("login body:", string(b))
	_ = resp.Body.Close()
}
