package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"sync"
	"vanya-and-co/services/auth/storage"
)

type AuthService struct {
	mu       sync.Mutex
	store    storage.UserStorage
	sessions map[string]string
}

func New(store storage.UserStorage) *AuthService {
	return &AuthService{
		store:    store,
		sessions: make(map[string]string),
	}
}

// Проверка токена (middleware)
func (s *AuthService) AuthFromRequest(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	username, ok := s.sessions[cookie.Value]
	return username, ok
}

// Вспомогательная функция для установки сессии
func (s *AuthService) setSession(w http.ResponseWriter, username string) {
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	s.sessions[token] = username

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
}

// Login - Вход
func (s *AuthService) Register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	u := r.FormValue("username")
	p := r.FormValue("password")
	isForm := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"

	if u == "" || p == "" {
		http.Error(w, "empty fields", 400)
		return
	}

	// Проверяем, существует ли пользователь
	_, err := s.store.GetUser(u)
	if err == nil {
		if isForm {
			msg := "пользователь с таким логином существует"
			http.Redirect(
				w, r,
				"/static/register-form.html?error="+url.QueryEscape(msg),
				http.StatusSeeOther,
			)
			return
		}
		http.Error(w, "user already exists", 409)
		return
	}

	// Создаём пользователя
	if err := s.store.CreateUser(u, p); err != nil {
		http.Error(w, "internal error", 500)
		return
	}

	s.setSession(w, u)

	if isForm {
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.Write([]byte("Created"))
}
func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}

	u := r.FormValue("username")
	p := r.FormValue("password")
	isForm := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"

	savedPwd, err := s.store.GetUser(u)
	if err != nil || savedPwd != p {
		if isForm {
			msg := "пользователя с таким паролем или логином не существует, попробуйте еще раз"
			http.Redirect(
				w, r,
				"/static/login-form.html?error="+url.QueryEscape(msg),
				http.StatusSeeOther,
			)
			return
		}
		http.Error(w, "unauthorized", 401)
		return
	}

	s.setSession(w, u)

	if isForm {
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.Write([]byte("OK"))
}

// Logout - Выход
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		delete(s.sessions, cookie.Value)
	}
	// Удаляем куку и редиректим на логин (Условие 2)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
