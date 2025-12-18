package storage

type UserStorage interface {
	CreateUser(login, password string) error
	GetUser(login string) (string, error)
}
