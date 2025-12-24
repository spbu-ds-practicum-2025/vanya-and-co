package main

import (
	"context"
	"log"

	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Подключаемся к File Service
	conn, err := grpc.Dial("localhost:5202",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := filepb.NewFileServiceClient(conn)

	// Тест 1: Загрузка файла
	log.Println("Testing Upload...")
	content := []byte("Hello, this is a test file!")
	uploadResp, err := client.Upload(context.Background(), &filepb.UploadRequest{
		Username: "testuser",
		Filename: "test.txt",
		Content:  content,
	})

	if err != nil {
		log.Printf("Upload failed: %v", err)
	} else {
		log.Printf("Upload successful: %v", uploadResp)
	}

	// Тест 2: Получение списка файлов
	log.Println("Testing List...")
	listResp, err := client.List(context.Background(), &filepb.ListRequest{
		Username: "testuser",
	})

	if err != nil {
		log.Printf("List failed: %v", err)
	} else {
		log.Printf("List successful. Files: %v", listResp.Files)
	}
}
