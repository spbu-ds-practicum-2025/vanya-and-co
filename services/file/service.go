package file

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type FileService struct {
	BasePath  string
	MaxSizeMB int
	Cluster   *ReplicaCluster
	db        *sql.DB
}

func New(base string, maxSizeMB int) *FileService {
	_ = os.MkdirAll(base, os.ModePerm)
	cluster := NewCluster(base, 3)
	dbPath := filepath.Join(base, "file.db")
	db, _ := sql.Open("sqlite", dbPath)
	// ignore error here; embedded services/tests will ensure path
	_ = os.MkdirAll(filepath.Dir(dbPath), os.ModePerm)
	if db != nil {
		_ = db.Ping()
		// create table if needed
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS files (id INTEGER PRIMARY KEY AUTOINCREMENT, owner TEXT, name TEXT, path TEXT, created INTEGER);`)
	}

	return &FileService{
		BasePath:  base,
		MaxSizeMB: maxSizeMB,
		Cluster:   cluster,
		db:        db,
	}
}

// Helper для генерации ID (замена uuid)
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// UPLOAD - Загрузка файла (Условие 4: изоляция)
func (s *FileService) Upload(w http.ResponseWriter, r *http.Request, username string) {
	// Ограничение размера
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.MaxSizeMB<<20))
	if err := r.ParseMultipartForm(int64(s.MaxSizeMB << 20)); err != nil {
		http.Error(w, "File too big", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "no file", 400)
		return
	}
	defer file.Close()

	// 1. Генерируем ID и чистим имя
	id := generateID()
	cleanName := filepath.Base(header.Filename)
	cleanName = strings.ReplaceAll(cleanName, " ", "_")
	filename := id + "-" + cleanName

	// 2. Путь: BasePath/username/filename
	userDir := filepath.Join(s.BasePath, filepath.Base(username))
	_ = os.MkdirAll(userDir, os.ModePerm)
	dst := filepath.Join(userDir, filename)

	// 3. Читаем в память для записи и репликации
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}

	// 4. Пишем на основной диск
	_ = os.WriteFile(dst, fileBytes, os.ModePerm)

	// store metadata in DB
	if s.db != nil {
		_, _ = s.db.Exec(`INSERT INTO files (owner, name, path, created) VALUES (?, ?, ?, ?)`, username, filename, dst, time.Now().Unix())
	}

	// 5. Репликация
	relPath := filepath.Join(filepath.Base(username), filename)
	s.Cluster.Write(relPath, fileBytes)

	// 6. Редирект обратно на список
	http.Redirect(w, r, "/files/list", http.StatusSeeOther)
}

// LIST - Список файлов (Условие 2 и 4)
func (s *FileService) List(w http.ResponseWriter, r *http.Request, username string) {
	// prefer DB-backed listing if available
	var rows *sql.Rows
	var err error
	if s.db != nil {
		rows, err = s.db.Query(`SELECT name FROM files WHERE owner = ? ORDER BY created DESC`, username)
	}
	var names []string
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				names = append(names, n)
			}
		}
	} else {
		userDir := filepath.Join(s.BasePath, filepath.Base(username))
		dirents, _ := os.ReadDir(userDir)
		for _, f := range dirents {
			if f.IsDir() {
				continue
			}
			names = append(names, f.Name())
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// HTML для отображения и кнопки "Выйти"
	fmt.Fprintf(w, `
		<html>
		<head>
			<title>Files</title>
			<style>
				body { font-family: sans-serif; padding: 20px; background:#f4f4f4; }
				.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
				table { width:100%%; border-collapse: collapse; background: white; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
				td, th { padding:12px; border-bottom:1px solid #ddd; text-align: left; }
				tr:hover { background:#f9f9f9; }
				.btn { padding: 5px 10px; text-decoration: none; color: white; border-radius: 4px; font-size: 14px; margin-right: 5px;}
				.dl { background: #2196F3; }
				.del { background: #F44336; }
				.logout-btn { background: #555; border: none; color: white; padding: 8px 15px; cursor: pointer; border-radius: 4px;}
				.upload-link { font-size: 16px; font-weight: bold; }
			</style>
		</head>
		<body>
			<div class="header">
				<div>
					<h1>Хранилище пользователя: %s</h1>
					<a href="/static/upload-form.html" class="upload-link">⬆ Загрузить новый файл</a>
				</div>
				<form action="/auth/logout" method="POST" style="margin:0;">
					<button type="submit" class="logout-btn">Выйти</button>
				</form>
			</div>

			<table>
				<tr><th>Имя файла</th><th style="text-align:right">Действия</th></tr>
	`, username)

	if len(names) == 0 {
		fmt.Fprintf(w, "<tr><td colspan='2' style='text-align:center; color:#777;'>Папка пуста</td></tr>")
	}

	for _, name := range names {
		// Ссылки на действия
		fmt.Fprintf(w, `
			<tr>
				<td style="word-break: break-all; max-width: 400px;">%s</td>
				<td style="min-width: 150px;">
					<div style="display: flex; flex-direction: column; gap: 5px; align-items: flex-end;">
						<a class="btn dl" href="/files/download?name=%s">Скачать</a>
						<a class="btn del" href="/files/delete?name=%s">Удалить</a>
					</div>
				</td>
			</tr>`, name, name, name)
	}

	fmt.Fprintf(w, `</table></body></html>`)
}

// DOWNLOAD (Условие 4: изоляция)
func (s *FileService) Download(w http.ResponseWriter, r *http.Request, username string) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}

	// Путь относительно корня хранилища: username/filename
	relPath := filepath.Join(filepath.Base(username), filepath.Base(name))
	path := filepath.Join(s.BasePath, relPath)

	// 1. Пробуем основной диск
	data, err := os.ReadFile(path)
	if err == nil {
		s.serveData(w, name, data)
		return
	}

	// 2. Если нет, ищем в кластере (репликах)
	if data, ok := s.Cluster.ReadAny(relPath); ok {
		s.serveData(w, name, data)
		return
	}

	http.Error(w, "File not found", 404)
}

// Вспомогательная функция отправки файла
func (s *FileService) serveData(w http.ResponseWriter, name string, data []byte) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	w.Write(data)
}

// DELETE (Условие 4: изоляция)
func (s *FileService) Delete(w http.ResponseWriter, r *http.Request, username string) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", 400)
		return
	}

	// Путь относительно корня хранилища: username/filename
	relPath := filepath.Join(filepath.Base(username), filepath.Base(name))

	// Удаляем с основного диска
	mainFile := filepath.Join(s.BasePath, relPath)
	_ = os.Remove(mainFile)

	// Удаляем с узлов
	s.Cluster.Delete(relPath)

	// delete metadata from DB
	if s.db != nil {
		_, _ = s.db.Exec(`DELETE FROM files WHERE owner = ? AND name = ?`, username, filepath.Base(name))
	}

	http.Redirect(w, r, "/files/list", http.StatusSeeOther)
}
