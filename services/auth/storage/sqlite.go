package storage

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		login TEXT PRIMARY KEY,
		password TEXT NOT NULL
	);
	`)
	if err != nil {
		return nil, err
	}

	return &SQLiteStorage{db: db}, nil
}

func (s *SQLiteStorage) CreateUser(login, password string) error {
	_, err := s.db.Exec(
		"INSERT INTO users(login, password) VALUES(?, ?)",
		login, password,
	)
	return err
}

func (s *SQLiteStorage) GetUser(login string) (string, error) {
	var password string
	err := s.db.QueryRow(
		"SELECT password FROM users WHERE login = ?",
		login,
	).Scan(&password)
	return password, err
}
