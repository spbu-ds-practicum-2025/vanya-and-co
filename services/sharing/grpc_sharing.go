package sharing

import (
	"context"
	"crypto/rand"
	"encoding/hex"

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

// Вспомогательная функция
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
