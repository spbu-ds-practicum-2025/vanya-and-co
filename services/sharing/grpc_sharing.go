package sharing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
)

// SharingServiceGRPC реализует gRPC интерфейс
type SharingServiceGRPC struct {
	sharingpb.UnimplementedSharingServiceServer
	service *SharingService
}

// NewGRPCService создает gRPC обертку
func NewGRPCService(service *SharingService) *SharingServiceGRPC {
	return &SharingServiceGRPC{service: service}
}

// ShareFile - gRPC метод для общего доступа к файлу
func (s *SharingServiceGRPC) ShareFile(ctx context.Context, req *sharingpb.ShareRequest) (*sharingpb.ShareResponse, error) {
	// Реализация
	token := generateToken()
	return &sharingpb.ShareResponse{
		Success: true,
		Token:   token,
	}, nil
}

// GetSharedFiles - gRPC метод для получения общих файлов
func (s *SharingServiceGRPC) GetSharedFiles(ctx context.Context, req *sharingpb.GetSharedRequest) (*sharingpb.GetSharedResponse, error) {
	// Получаем все ссылки пользователя из базы данных
	var files []*sharingpb.SharedFileInfo

	if s.service.db != nil {
		rows, err := s.service.db.Query(`SELECT token, file, expires FROM links WHERE owner = ?`, req.Username)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var token, filename string
				var expires int64
				if rows.Scan(&token, &filename, &expires) == nil {
					files = append(files, &sharingpb.SharedFileInfo{
						Filename: filename,
						Owner:    req.Username,
						SharedAt: expires,
					})
				}
			}
		}
	}

	return &sharingpb.GetSharedResponse{
		Files: files,
	}, nil
}

// CreateLink - gRPC метод для создания публичной ссылки
func (s *SharingServiceGRPC) CreateLink(ctx context.Context, req *sharingpb.CreateLinkRequest) (*sharingpb.CreateLinkResponse, error) {
	token := generateToken()
	expires := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second)

	link := Link{
		Token:   token,
		Owner:   req.Owner,
		File:    req.Filename,
		Expires: expires,
	}

	s.service.mu.Lock()
	s.service.links[token] = link
	s.service.mu.Unlock()

	if s.service.db != nil {
		_, _ = s.service.db.Exec(`INSERT INTO links (token, owner, file, expires) VALUES (?, ?, ?, ?)`,
			token, req.Owner, req.Filename, expires.Unix())
	}

	return &sharingpb.CreateLinkResponse{
		Link:      fmt.Sprintf("/share/%s/download", token),
		Token:     token,
		ExpiresAt: expires.Unix(),
	}, nil
}

// Вспомогательная функция
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
