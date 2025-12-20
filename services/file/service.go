package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	"google.golang.org/grpc"
)

// FileService представляет сервис для работы с файлами
type FileService struct {
	storagePath string
}

// NewFileService создает новый экземпляр FileService
func NewFileService() *FileService {
	return &FileService{
		storagePath: "./storage",
	}
}

// SaveFile сохраняет файл
func (s *FileService) SaveFile(filename string, content []byte) (string, error) {
	// Создаем директорию, если её нет
	if err := os.MkdirAll(s.storagePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %v", err)
	}

	// Генерируем уникальный ID
	fileID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
	filePath := filepath.Join(s.storagePath, fileID)

	// Сохраняем файл
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return fileID, nil
}

// GetFile возвращает файл по ID
func (s *FileService) GetFile(fileID string) ([]byte, string, error) {
	filePath := filepath.Join(s.storagePath, fileID)

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("file not found: %s", fileID)
	}

	// Читаем файл
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %v", err)
	}

	return content, fileID, nil
}

// ListFiles возвращает список файлов
func (s *FileService) ListFiles() ([]*filepb.FileInfo, error) {
	// Читаем все файлы из storage
	files, err := os.ReadDir(s.storagePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Если директории нет, возвращаем пустой список
			return []*filepb.FileInfo{}, nil
		}
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	var fileInfos []*filepb.FileInfo
	for _, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				continue
			}

			fileInfos = append(fileInfos, &filepb.FileInfo{
				Id:         file.Name(),
				Filename:   file.Name(),
				Size:       info.Size(),
				UploadedAt: info.ModTime().Unix(),
			})
		}
	}

	return fileInfos, nil
}

// FileServiceGRPC реализует gRPC интерфейс
type FileServiceGRPC struct {
	filepb.UnimplementedFileServiceServer
	service *FileService
}

// NewGRPCService создает gRPC обертку
func NewGRPCService(service *FileService) *FileServiceGRPC {
	return &FileServiceGRPC{service: service}
}

// List - gRPC метод для получения списка файлов
func (s *FileServiceGRPC) List(ctx context.Context, req *filepb.ListFilesRequest) (*filepb.ListFilesResponse, error) {
	fmt.Printf("List request for user: %s\n", req.Username)

	files, err := s.service.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	return &filepb.ListFilesResponse{
		Files: files,
	}, nil
}

// Upload - gRPC метод для загрузки файла
func (s *FileServiceGRPC) Upload(ctx context.Context, req *filepb.UploadRequest) (*filepb.UploadResponse, error) {
	fmt.Printf("Upload request: user=%s, filename=%s, size=%d\n",
		req.Username, req.Filename, len(req.Content))

	fileID, err := s.service.SaveFile(req.Filename, req.Content)
	if err != nil {
		return &filepb.UploadResponse{
			Success: false,
			Message: fmt.Sprintf("Upload failed: %v", err),
		}, nil
	}

	return &filepb.UploadResponse{
		Success: true,
		FileId:  fileID,
		Message: "File uploaded successfully",
	}, nil
}

// Download - gRPC метод для скачивания файла
func (s *FileServiceGRPC) Download(ctx context.Context, req *filepb.DownloadRequest) (*filepb.DownloadResponse, error) {
	fmt.Printf("Download request: file_id=%s, user=%s\n",
		req.FileId, req.Username)

	content, filename, err := s.service.GetFile(req.FileId)
	if err != nil {
		return &filepb.DownloadResponse{
			Success: false,
			Message: fmt.Sprintf("Download failed: %v", err),
		}, nil
	}

	return &filepb.DownloadResponse{
		Success:  true,
		Filename: filename,
		Content:  content,
		Message:  "File downloaded successfully",
	}, nil
}

// RegisterService регистрирует gRPC сервис
func (s *FileServiceGRPC) RegisterService(server *grpc.Server) {
	filepb.RegisterFileServiceServer(server, s)
}
