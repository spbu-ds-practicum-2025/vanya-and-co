package file

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/proto"
	"google.golang.org/grpc"
	_ "modernc.org/sqlite"
)

// FileMetadata представляет метаданные файла
type FileMetadata struct {
	ID         string    `json:"id"`
	Filename   string    `json:"filename"`
	Owner      string    `json:"owner"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// FileService представляет сервис для работы с файлами
type FileService struct {
	storagePath string
	db          *sql.DB
	cluster     *ReplicaCluster
}

// NewFileService создает новый экземпляр FileService
func NewFileService() *FileService {
	// Используем переменную окружения или путь по умолчанию
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		// Для локального запуска используем ./storage относительно корня проекта
		if cwd, err := os.Getwd(); err == nil {
			// Если мы в services/file/cmd/server, поднимаемся на 3 уровня вверх
			if strings.Contains(cwd, "services/file/cmd/server") {
				storagePath = filepath.Join(cwd, "../../../storage")
			} else if strings.Contains(cwd, "services/file") {
				// Если мы в любой подпапке services/file, поднимаемся на 2 уровня вверх
				storagePath = filepath.Join(cwd, "../../storage")
			} else {
				storagePath = filepath.Join(cwd, "storage")
			}
		} else {
			storagePath = "./storage"
		}
	}

	absolutePath, err := filepath.Abs(storagePath)
	if err != nil {
		log.Fatalf("Failed to resolve storage path: %v", err)
	}
	log.Printf("File service using storage path: %s (resolved to: %s)", storagePath, absolutePath)

	// Инициализируем базу данных
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		// Для локального запуска используем путь относительно корня проекта
		if cwd, err := os.Getwd(); err == nil {
			if strings.Contains(cwd, "services/file/cmd/server") {
				dbPath = filepath.Join(cwd, "../../../services/file/data/file.db")
			} else {
				dbPath = filepath.Join(cwd, "services/file/data/file.db")
			}
		} else {
			dbPath = "services/file/data/file.db"
		}
	}
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	log.Printf("File service using database path: %s", dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open file database: %v", err)
	}

	// Создаем кластер реплик
	cluster := NewCluster(absolutePath, 3) // 3 реплики

	service := &FileService{
		storagePath: absolutePath,
		db:          db,
		cluster:     cluster,
	}

	// Инициализируем базу данных
	if err := service.migrate(); err != nil {
		log.Fatalf("Failed to migrate file database: %v", err)
	}

	return service
}

// migrate инициализирует базу данных
func (s *FileService) migrate() error {
	// Создаем таблицу с правильной схемой
	createQuery := `
		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			owner TEXT NOT NULL,
			size INTEGER NOT NULL,
			uploaded_at INTEGER NOT NULL
		);
	`
	if _, err := s.db.Exec(createQuery); err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Проверяем и обновляем схему таблицы, если необходимо
	alterQueries := []string{
		`ALTER TABLE files ADD COLUMN uploaded_at INTEGER DEFAULT 0;`,
	}

	for _, query := range alterQueries {
		if _, err := s.db.Exec(query); err != nil {
			// Игнорируем ошибки "duplicate column name" - колонка уже существует
			if !strings.Contains(err.Error(), "duplicate column name") &&
				!strings.Contains(err.Error(), "already exists") {
				log.Printf("Warning: failed to alter table (%s): %v", query, err)
			}
		}
	}

	// Создаем индексы (только после создания таблицы)
	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_files_owner ON files(owner);`,
		`CREATE INDEX IF NOT EXISTS idx_files_uploaded_at ON files(uploaded_at);`,
	}

	for _, query := range indexQueries {
		if _, err := s.db.Exec(query); err != nil {
			log.Printf("Warning: failed to create index (%s): %v", query, err)
			// Не возвращаем ошибку, так как индекс может уже существовать
		}
	}

	log.Printf("Database migration completed successfully")
	return nil
}

// SaveFile сохраняет файл
func (s *FileService) SaveFile(owner, filename string, content []byte) (string, error) {
	log.Printf("SaveFile called: owner=%s, filename=%s, size=%d", owner, filename, len(content))

	// Проверяем существование storage path
	if _, err := os.Stat(s.storagePath); os.IsNotExist(err) {
		log.Printf("Storage path does not exist: %s", s.storagePath)
		// Пытаемся создать
		if err := os.MkdirAll(s.storagePath, 0755); err != nil {
			log.Printf("Failed to create storage path: %v", err)
			return "", fmt.Errorf("failed to create storage: %v", err)
		}
		log.Printf("Created storage path: %s", s.storagePath)
	}

	// Генерируем уникальный ID
	fileID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
	log.Printf("Generated fileID: %s", fileID)

	// Сохраняем файл через кластер реплик
	relPath := filepath.Join(owner, fileID)
	log.Printf("Relative path for cluster: %s", relPath)

	// Проверяем кластер
	if s.cluster == nil {
		log.Printf("ERROR: Cluster is nil!")
		return "", fmt.Errorf("cluster not initialized")
	}

	log.Printf("Writing to cluster...")
	s.cluster.Write(relPath, content)

	// Сохраняем метаданные в базу данных
	now := time.Now()
	log.Printf("Saving to database...")

	// Проверяем соединение с БД
	if err := s.db.Ping(); err != nil {
		log.Printf("Database ping failed: %v", err)
		return "", fmt.Errorf("database connection failed: %v", err)
	}

	// Вставляем данные
	result, err := s.db.Exec(
		`INSERT INTO files (id, filename, owner, size, uploaded_at) VALUES (?, ?, ?, ?, ?)`,
		fileID, filename, owner, len(content), now.Unix(),
	)

	if err != nil {
		log.Printf("Failed to save file metadata: %v", err)
		return "", fmt.Errorf("failed to save file metadata: %v", err)
	}

	rows, _ := result.RowsAffected()
	log.Printf("File saved successfully: owner=%s, filename=%s, id=%s, size=%d, rows=%d",
		owner, filename, fileID, len(content), rows)

	return fileID, nil
}

// ListFiles возвращает список файлов пользователя
func (s *FileService) ListFiles(owner string) ([]*filepb.FileInfo, error) {
	rows, err := s.db.Query(`
		SELECT id, filename, size, uploaded_at
		FROM files
		WHERE owner = ?
		ORDER BY uploaded_at DESC
	`, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to query files: %v", err)
	}
	defer rows.Close()

	var fileInfos []*filepb.FileInfo
	for rows.Next() {
		var id, filename string
		var size int64
		var uploadedAt int64

		if err := rows.Scan(&id, &filename, &size, &uploadedAt); err != nil {
			continue
		}

		fileInfos = append(fileInfos, &filepb.FileInfo{
			Name:    id, // Используем ID как имя файла для API
			Size:    size,
			Created: uploadedAt,
		})
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

	files, err := s.service.ListFiles(req.Username)
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
	fmt.Printf("Download request: filename=%s, user=%s\n",
		req.Filename, req.Username)

	content, filename, err := s.service.GetFile(req.Username, req.Filename)
	if err != nil {
		return nil, fmt.Errorf("download failed: %v", err)
	}

	return &filepb.DownloadResponse{
		Content:  content,
		Filename: filename,
	}, nil
}

// DeleteFile удаляет файл
func (s *FileService) DeleteFile(owner, fileID string) error {
	// Удаляем из кластера реплик
	relPath := filepath.Join(owner, fileID)
	s.cluster.Delete(relPath)

	// Удаляем метаданные из базы данных
	_, err := s.db.Exec(`DELETE FROM files WHERE id = ? AND owner = ?`, fileID, owner)
	if err != nil {
		return fmt.Errorf("failed to delete file metadata: %v", err)
	}

	return nil
}

// Delete - gRPC метод для удаления файла
func (s *FileServiceGRPC) Delete(ctx context.Context, req *filepb.DeleteRequest) (*filepb.DeleteResponse, error) {
	fmt.Printf("Delete request: filename=%s, user=%s\n",
		req.Filename, req.Username)

	err := s.service.DeleteFile(req.Username, req.Filename)
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
