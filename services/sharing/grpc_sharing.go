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
	// Реализация
	return &sharingpb.GetSharedResponse{
		Files: []*sharingpb.SharedFileInfo{},
	}, nil
}

// CreateLink - gRPC метод для создания публичной ссылки
func (s *SharingServiceGRPC) CreateLink(ctx context.Context, req *sharingpb.CreateLinkRequest) (*sharingpb.CreateLinkResponse, error) {
	token := generateToken()
	expiresAt := time.Now().Add(time.Duration(req.TtlSeconds) * time.Second).Unix()

	// Сохраняем ссылку в сервисе
	link := Link{
		Token:   token,
		Owner:   req.Owner,
		File:    req.Filename,
		Expires: time.Unix(expiresAt, 0),
	}

	s.service.mu.Lock()
	s.service.links[token] = link
	s.service.mu.Unlock()

	if s.service.db != nil {
		_, _ = s.service.db.Exec(`INSERT INTO links (token, owner, file, expires) VALUES (?, ?, ?, ?)`,
			token, req.Owner, req.Filename, expiresAt)
	}

	return &sharingpb.CreateLinkResponse{
		Link:      fmt.Sprintf("/share/%s", token),
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// Вспомогательная функция
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
