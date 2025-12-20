package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	filepb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/file/filepb"
	sharingpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/sharing/sharingpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	authClient    authpb.AuthClient
	fileClient    filepb.FileServiceClient
	sharingClient sharingpb.SharingServiceClient
}

func NewGateway() (*Gateway, error) {
	authConn, err := grpc.Dial("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	fileConn, err := grpc.Dial("localhost:8082", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	sharingConn, err := grpc.Dial("localhost:8083", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &Gateway{
		authClient:    authpb.NewAuthClient(authConn),
		fileClient:    filepb.NewFileServiceClient(fileConn),
		sharingClient: sharingpb.NewSharingServiceClient(sharingConn),
	}, nil
}

func (g *Gateway) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "unauthorized", 401)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := g.authClient.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: cookie.Value})
		if err != nil || resp.Username == "" {
			http.Error(w, "unauthorized", 401)
			return
		}

		ctx = context.WithValue(r.Context(), "username", resp.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (g *Gateway) handleFileList(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value("username").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.fileClient.List(ctx, &filepb.ListFilesRequest{Username: username})
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp.Files)
}

func (g *Gateway) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20) // 10 MB limit
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	username := r.Context().Value("username").(string)
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := g.fileClient.Upload(ctx, &filepb.UploadRequest{
		Username: username,
		Filename: handler.Filename,
		Content:  content,
	})
	if err != nil || !resp.Success {
		http.Error(w, "Failed to upload file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded successfully"))
}

func main() {
	gateway, err := NewGateway()
	if err != nil {
		log.Fatalf("Failed to create gateway: %v", err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../static"))))
	http.HandleFunc("/api/files/list", gateway.authMiddleware(gateway.handleFileList))
	http.HandleFunc("/files/upload", gateway.authMiddleware(gateway.handleFileUpload))

	http.Handle("/", http.FileServer(http.Dir("../../static")))

	log.Println("Gateway starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
