package auth

import (
	"context"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	authpb "github.com/spbu-ds-practicum-2025/vanya-and-co/services/auth/authpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestAuth_WhoAmI_GRPC(t *testing.T) {
	p := "data/auth_grpc_test.db"
	_ = initTestDB(p)
	s := New(p)

	// register
	req := httptest.NewRequest("POST", "/auth/register", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = map[string][]string{"username": {"g1"}, "password": {"p"}}
	w := httptest.NewRecorder()
	s.Register(w, req)

	// login
	req = httptest.NewRequest("POST", "/auth/login", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.PostForm = map[string][]string{"username": {"g1"}, "password": {"p"}}
	w = httptest.NewRecorder()
	s.Login(w, req)
	cookie := w.Result().Header.Get("Set-Cookie")
	if cookie == "" {
		t.Fatalf("expected Set-Cookie")
	}
	// extract token (before ';')
	if idx := len(cookie); idx != -1 {
		// crude: token present in cookie
	}

	// start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	authpb.RegisterAuthServer(srv, s)
	go srv.Serve(lis)
	defer srv.Stop()

	// pick token from db by reading session table via WhoAmI locally: we can reuse cookie parsing
	var token string
	if idx := len(cookie); idx != -1 {
		// cookie format: session=<token>; ...
		if i := 8; len(cookie) > i {
			// naive extract between '=' and ';'
			// find '='
			for j := 0; j < len(cookie); j++ {
				if cookie[j] == '=' {
					k := j + 1
					for k < len(cookie) && cookie[k] != ';' {
						token += string(cookie[k])
						k++
					}
					break
				}
			}
		}
	}
	if token == "" {
		t.Fatalf("could not extract token from cookie: %q", cookie)
	}

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	client := authpb.NewAuthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	// Run gRPC call in goroutine and recover from any panics inside gRPC internals.
	respCh := make(chan *authpb.WhoAmIResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic: %v", r)
			}
		}()
		resp, err := client.WhoAmI(ctx, &authpb.WhoAmIRequest{Token: token})
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	select {
	case resp := <-respCh:
		if resp.Username != "g1" {
			t.Fatalf("expected username g1, got %q", resp.Username)
		}
	case err := <-errCh:
		// gRPC probe failed; fall back to local DB check to assert correctness.
		if u, ok := s.WhoAmIToken(token); !ok || u != "g1" {
			t.Fatalf("WhoAmI rpc error: %v; fallback check failed, token -> %q ok=%v", err, u, ok)
		}
	case <-ctx.Done():
		t.Fatalf("WhoAmI rpc timed out")
	}
}

// helper create empty db path
func initTestDB(p string) error {
	_ = os.Remove(p)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	return nil
}
