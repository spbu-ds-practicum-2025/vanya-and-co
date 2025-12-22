package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"context"

	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	_ "modernc.org/sqlite"
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthService struct {
	mu       sync.Mutex
	db       *sql.DB
	path     string               // путь к файлу auth.db
	sessions map[string]time.Time // token -> expires
}

func New(dbPath string) *AuthService {
	if dbPath == "" {
		cwd, _ := os.Getwd()
		dbPath = filepath.Join(cwd, "services", "auth", "data", "auth.db")
	}
	_ = os.MkdirAll(filepath.Dir(dbPath), 0o755)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	s := &AuthService{db: db, path: dbPath, sessions: make(map[string]time.Time)}
	if err := s.migrate(); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	return s
}

// WhoAmIToken looks up username by session token
func (s *AuthService) WhoAmIToken(token string) (string, bool) {
	var username string
	var exp int64
	if err := s.db.QueryRow(`SELECT username, expires FROM sessions WHERE token = ?`, token).Scan(&username, &exp); err != nil {
		return "", false
	}
	if time.Now().Unix() > exp {
		return "", false
	}
	return username, true
}

// Implement gRPC server for Auth
func (s *AuthService) WhoAmI(ctx context.Context, req *authpb.WhoAmIRequest) (*authpb.WhoAmIResponse, error) {
	if req == nil || req.Token == "" {
		return &authpb.WhoAmIResponse{Username: ""}, nil
	}
	if u, ok := s.WhoAmIToken(req.Token); ok {
		return &authpb.WhoAmIResponse{Username: u}, nil
	}
	return &authpb.WhoAmIResponse{Username: ""}, nil
}

// HTTP WhoAmI handler для проверки токенов
func (s *AuthService) WhoAmIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "Token required", http.StatusBadRequest)
		return
	}

	if u, ok := s.WhoAmIToken(token); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"username": "` + u + `"}`))
		return
	}

	http.Error(w, "Invalid token", http.StatusUnauthorized)
}

func (s *AuthService) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (username TEXT PRIMARY KEY, password TEXT);`,
		`CREATE TABLE IF NOT EXISTS sessions (token TEXT PRIMARY KEY, username TEXT, expires INTEGER);`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Проверка токена (middleware)
func (s *AuthService) AuthFromRequest(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}
	var username string
	var exp int64
	err = s.db.QueryRow(`SELECT username, expires FROM sessions WHERE token = ?`, cookie.Value).Scan(&username, &exp)
	if err != nil {
		return "", false
	}
	if time.Now().Unix() > exp {
		return "", false
	}
	return username, true
}

// Вспомогательная функция для установки сессии
func (s *AuthService) setSession(w http.ResponseWriter, username string) {
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().Add(24 * time.Hour)
	
	log.Printf("[Auth] Creating session for user: %s", username)
	
	if _, err := s.db.Exec(`INSERT INTO sessions (token, username, expires) VALUES (?, ?, ?)`, token, username, expires.Unix()); err != nil {
		log.Println("[Auth] setSession insert error:", err)
	} else {
		log.Printf("[Auth] Session saved to DB for user %s", username)
	}
	
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", Expires: expires, HttpOnly: true})
}

// Login - Вход
func (s *AuthService) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var u, p string
	isForm := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
	if isForm {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		u = r.FormValue("username")
		p = r.FormValue("password")
	} else if r.Header.Get("Content-Type") == "application/json" {
		var body struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		u = body.Username
		p = body.Password
	} else {
		// try form as fallback
		_ = r.ParseForm()
		u = r.FormValue("username")
		p = r.FormValue("password")
	}

	var savedPwd string
	err := s.db.QueryRow(`SELECT password FROM users WHERE username = ?`, u).Scan(&savedPwd)
	exists := err == nil

	// Условие 3: Ошибка, если логин не найден или пароль неверен
	if !exists || savedPwd != p {
		log.Printf("[Auth] Login failed for user: %s", u)
		if isForm {
			msg := "Пользователя с указанным логином не существует, либо неверный пароль"
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
		log.Printf("[Auth] Redirecting user %s to /files/list", u)
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.Write([]byte("OK"))
}

// Register - Регистрация
func (s *AuthService) Register(w http.ResponseWriter, r *http.Request) {
	var u, p string
	isForm := r.Header.Get("Content-Type") == "application/x-www-form-urlencoded"
	if isForm {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		u = r.FormValue("username")
		p = r.FormValue("password")
	} else if r.Header.Get("Content-Type") == "application/json" {
		var body struct{ Username, Password string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", 400)
			return
		}
		u = body.Username
		p = body.Password
	} else {
		_ = r.ParseForm()
		u = r.FormValue("username")
		p = r.FormValue("password")
	}

	if u == "" || p == "" {
		http.Error(w, "empty fields", 400)
		return
	}

	var tmp string
	err := s.db.QueryRow(`SELECT username FROM users WHERE username = ?`, u).Scan(&tmp)
	exists := err == nil

	// Условие 3: Ошибка, если пользователь уже существует
	if exists {
		log.Printf("[Auth] Registration failed: user %s already exists", u)
		if isForm {
			msg := "Пользователь с указанным вами логином существует"
			http.Redirect(w, r, "/static/register-form.html?error="+url.QueryEscape(msg), http.StatusSeeOther)
			return
		}
		http.Error(w, "user already exists", 409)
		return
	}

	// Создаем пользователя
	if _, err := s.db.Exec(`INSERT INTO users (username, password) VALUES (?, ?)`, u, p); err != nil {
		log.Printf("[Auth] Failed to create user %s: %v", u, err)
		http.Error(w, "internal", 500)
		return
	}
	
	log.Printf("[Auth] User %s registered successfully", u)

	// Успешная авторизация (Условие 3)
	s.setSession(w, u)

	// Условие 1: Сразу перекидываем на файлы
	if isForm {
		log.Printf("[Auth] Redirecting new user %s to /files/list", u)
		http.Redirect(w, r, "/files/list", http.StatusSeeOther)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Created"))
}

// Logout - Выход
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		// remove session from DB
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, cookie.Value)
		log.Printf("[Auth] Session deleted")
	}
	// Удаляем куку и редиректим на логин (Условие 2)
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
