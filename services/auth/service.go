package auth

import (
  "crypto/sha256"
  "encoding/hex"
  "encoding/json"
  "net/http"
  "sync"
)

type User struct {
  Username string 'json:"username"'
  Password string 'json:"password"'
}

type AuthService struct {
  mu sync.Mutex
  users map[string]string
}
