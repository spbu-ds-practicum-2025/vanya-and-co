package storage

import "errors"

type MemoryStorage struct {
	users map[string]string
}

func NewMemory() *MemoryStorage {
	return &MemoryStorage{
		users: make(map[string]string),
	}
}

func (m *MemoryStorage) CreateUser(login, password string) error {
	if _, exists := m.users[login]; exists {
		return errors.New("user already exists")
	}
	m.users[login] = password
	return nil
}

func (m *MemoryStorage) GetUser(login string) (string, error) {
	pwd, exists := m.users[login]
	if !exists {
		return "", errors.New("user not found")
	}
	return pwd, nil
}
