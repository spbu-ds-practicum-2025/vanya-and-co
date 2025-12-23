package file

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/proto"
	"google.golang.org/grpc"
)

// FileService представляет сервис для работы с файлами
type FileService struct {
	storagePath string
}

// NewFileService создает новый экземпляр FileService
func NewFileService() *FileService {
	absolutePath, err := filepath.Abs("./storage")
	if err != nil {
		log.Fatalf("Failed to resolve storage path: %v", err)
	}
	return &FileService{
		storagePath: absolutePath,
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
				Name:    file.Name(),
				Size:    info.Size(),
				Created: info.ModTime().Unix(),
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
func (s *FileServiceGRPC) List(ctx context.Context, req *filepb.ListRequest) (*filepb.ListResponse, error) {
	fmt.Printf("List request for user: %s\n", req.Username)

	files, err := s.service.ListFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}

	return &filepb.ListResponse{
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
	fmt.Printf("Download request: filename=%s, user=%s\n",
		req.Filename, req.Username)

	content, filename, err := s.service.GetFile(req.Filename)
	if err != nil {
		return nil, fmt.Errorf("download failed: %v", err)
	}

	return &filepb.DownloadResponse{
		Content:  content,
		Filename: filename,
	}, nil
}

// DeleteFile удаляет файл
func (s *FileService) DeleteFile(fileID string) error {
	filePath := filepath.Join(s.storagePath, fileID)

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", fileID)
	}

	// Удаляем файл
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}

	return nil
}

// Delete - gRPC метод для удаления файла
func (s *FileServiceGRPC) Delete(ctx context.Context, req *filepb.DeleteRequest) (*filepb.DeleteResponse, error) {
	fmt.Printf("Delete request: filename=%s, user=%s\n",
		req.Filename, req.Username)

	// В текущей реализации просто удаляем файл по имени
	// В будущем можно добавить проверку прав доступа
	err := s.service.DeleteFile(req.Filename)
	if err != nil {
		return nil, fmt.Errorf("delete failed: %v", err)
	}

	return &filepb.DeleteResponse{
		Success: true,
	}, nil
}

// RegisterService регистрирует gRPC сервис
func (s *FileServiceGRPC) RegisterService(server *grpc.Server) {
	filepb.RegisterFileServiceServer(server, s)
}
