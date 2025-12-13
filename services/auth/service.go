package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthService struct {
	mu       sync.Mutex
	users    map[string]string // username -> password
	path     string            // путь к файлу users.json
	sessions map[string]string // token -> username
}

func New(usersPath string) *AuthService {
	if usersPath == "" {
		cwd, _ := os.Getwd()
		usersPath = filepath.Join(cwd, "services", "auth", "data", "users.json")
	}

	s := &AuthService{
		users:    make(map[string]string),
		path:     usersPath,
		sessions: make(map[string]string),
	}
	
	_ = os.MkdirAll(filepath.Dir(usersPath), 0o755)
	s.load()
	return s
}

func (s *AuthService) load() {
	f, err := os.Open(s.path)
	if err != nil {
		return
	}
	defer f.Close()
	var users []User
	if err := json.NewDecoder(f).Decode(&users); err == nil {
		for _, u := range users {
			s.users[u.Username] = u.Password
		}
	}
}

func (s *AuthService) save() {
	s.mu.Lock()
	defer s.mu.Unlock()
	var users []User
	for u, p := range s.users {
		users = append(users, User{Username: u, Password: p})
	}
	f, err := os.Create(s.path)
	if err != nil {
		log.Println("Error saving users:", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(users)
}

// Проверка токена (middleware)
func (s *AuthService) AuthFromRequest(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	s.mu.Lock()
	username, ok := s.sessions[cookie.Value]
	s.mu.Unlock()
	return username, ok
}

// Вспомогательная функция для установки сессии
func (s *AuthService) setSession(w http.ResponseWriter, username string) {
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	s.mu.Lock()
	s.sessions[token] = username
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/", 
		HttpOnly: true,
	})
}

// Login - Вход
func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	u := r.FormValue("username")
	p := r.FormValue("password")
	isForm := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"

	s.mu.Lock()
	savedPwd, exists := s.users[u]
	s.mu.Unlock()

	// Условие 3: Ошибка, если логин не найден или пароль неверен
	if !exists || savedPwd != p {
		if isForm {
			msg := "пользователя с таким паролем или логином не существует, попробуйте еще раз"
			// Редирект на форму с ошибкой и очисткой полей
			http.Redirect(w, r, "/static/login-form.html?error="+url.QueryEscape(msg), http.StatusSeeOther) 
			return
		}
		http.Error(w, "unauthorized", 401)
		return
	}

	// Успешный вход (Условие 3)
	s.setSession(w, u)

	// Условие 1: Сразу перекидываем на файлы
	if isForm {
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.Write([]byte("OK"))
}

// Register - Регистрация
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

	s.mu.Lock()
	_, exists := s.users[u]
	s.mu.Unlock()

	// Условие 3: Ошибка, если пользователь уже существует
	if exists {
		if isForm {
			msg := "пользователь с таким логином существует"
			// Редирект на форму с ошибкой
			http.Redirect(w, r, "/static/register-form.html?error="+url.QueryEscape(msg), http.StatusSeeOther)
			return
		}
		http.Error(w, "user already exists", 409)
		return
	}

	// Создаем пользователя
	s.mu.Lock()
	s.users[u] = p
	s.mu.Unlock()
	s.save()

	// Успешная авторизация (Условие 3)
	s.setSession(w, u)

	// Условие 1: Сразу перекидываем на файлы
	if isForm {
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.Write([]byte("Created"))
}

// Logout - Выход
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	// Удаляем куку и редиректим на логин (Условие 2)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}