package file

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	absolutePath, err := filepath.Abs("./data")
	if err != nil {
		log.Fatalf("Failed to resolve storage path: %v", err)
	}
	return &FileService{
		storagePath: absolutePath,
	}
}

// SaveFile сохраняет файл для указанного пользователя
func (s *FileService) SaveFile(username, filename string, content []byte) (string, error) {
	// Создаем директорию пользователя
	userPath := filepath.Join(s.storagePath, username)
	if err := os.MkdirAll(userPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create user directory: %v", err)
	}

	// Генерируем уникальный ID
	fileID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
	filePath := filepath.Join(userPath, fileID)

	// Сохраняем файл
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return "", fmt.Errorf("failed to save file: %v", err)
	}

	return fileID, nil
}

// GetFile возвращает файл по ID для указанного пользователя
func (s *FileService) GetFile(username, fileID string) ([]byte, string, error) {
	userPath := filepath.Join(s.storagePath, username)
	filePath := filepath.Join(userPath, fileID)

	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("file not found: %s", fileID)
	}

	// Читаем файл
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file: %v", err)
	}

	// Извлекаем реальное имя файла из fileID (формат: timestamp_filename)
	parts := strings.Split(fileID, "_")
	filename := fileID
	if len(parts) > 1 {
		// Убираем timestamp и соединяем оставшиеся части
		filename = strings.Join(parts[1:], "_")
	}

	return content, filename, nil
}

// DeleteFile удаляет файл пользователя
func (s *FileService) DeleteFile(username, fileID string) error {
	userPath := filepath.Join(s.storagePath, username)
	filePath := filepath.Join(userPath, fileID)

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

// ListFiles возвращает список файлов для указанного пользователя
func (s *FileService) ListFiles(username string) ([]*filepb.FileInfo, error) {
	userPath := filepath.Join(s.storagePath, username)

	// Читаем файлы пользователя
	files, err := os.ReadDir(userPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Если директории пользователя нет, возвращаем пустой список
			return []*filepb.FileInfo{}, nil
		}
		return nil, fmt.Errorf("failed to list files for user %s: %v", username, err)
	}

	var fileInfos []*filepb.FileInfo
	for _, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				continue
			}

			// Извлекаем реальное имя файла из fileID
			fileID := file.Name()
			filename := fileID
			parts := strings.Split(fileID, "_")
			if len(parts) > 1 {
				filename = strings.Join(parts[1:], "_")
			}

			fileInfos = append(fileInfos, &filepb.FileInfo{
				Id:         fileID,
				Filename:   filename,
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

	files, err := s.service.ListFiles(req.Username)
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

	fileID, err := s.service.SaveFile(req.Username, req.Filename, req.Content)
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

	content, filename, err := s.service.GetFile(req.Username, req.FileId)
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

// Delete - gRPC метод для удаления файла
func (s *FileServiceGRPC) Delete(ctx context.Context, req *filepb.DeleteRequest) (*filepb.DeleteResponse, error) {
	fmt.Printf("Delete request: file_id=%s, user=%s\n",
		req.FileId, req.Username)

	err := s.service.DeleteFile(req.Username, req.FileId)
	if err != nil {
		return &filepb.DeleteResponse{
			Success: false,
			Message: fmt.Sprintf("Delete failed: %v", err),
		}, nil
	}

	return &filepb.DeleteResponse{
		Success: true,
		Message: "File deleted successfully",
	}, nil
}

// RegisterService регистрирует gRPC сервис
func (s *FileServiceGRPC) RegisterService(server *grpc.Server) {
	filepb.RegisterFileServiceServer(server, s)
}
