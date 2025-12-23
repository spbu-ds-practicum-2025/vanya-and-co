package main

import (
	"context"
	"fmt"
	"log"

	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Подключаемся к gRPC серверу auth
	conn, err := grpc.Dial("localhost:5101", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := authpb.NewAuthClient(conn)

	// Тестируем WhoAmI с пустым токеном
	req := &authpb.WhoAmIRequest{
		Token: "",
	}
	log.Printf("Created request: token=%q", req.Token)
	resp, err := client.WhoAmI(context.Background(), req)
	if err != nil {
		log.Printf("WhoAmI with empty token error: %v", err)
	} else {
		log.Printf("WhoAmI with empty token result: username=%q", resp.Username)
	}

	// Тестируем WhoAmI с некорректным токеном
	resp, err = client.WhoAmI(context.Background(), &authpb.WhoAmIRequest{Token: "invalid-token"})
	if err != nil {
		log.Printf("WhoAmI with invalid token error: %v", err)
	} else {
		log.Printf("WhoAmI with invalid token result: username=%q", resp.Username)
	}

	// Тестируем WhoAmI с nil request
	resp, err = client.WhoAmI(context.Background(), nil)
	if err != nil {
		log.Printf("WhoAmI with nil request error: %v", err)
	} else {
		log.Printf("WhoAmI with nil request result: username=%q", resp.Username)
	}

	fmt.Println("Auth testing completed")
}
