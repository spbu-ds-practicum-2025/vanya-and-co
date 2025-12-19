Running with Docker

1. Build and start services (requires Docker & Docker Compose):

```bash
docker-compose up --build
```

Services (defaults):
- `auth` HTTP: 5100, gRPC: 5101
- `file` HTTP: 5200
- `sharing` HTTP: 5300
- `gateway` HTTP: 8080 (gateway is configured to call `auth` gRPC at `auth:5101` inside compose)

2. Open the gateway in your browser: http://localhost:8080

Notes:
- Auth exposes both HTTP endpoints (`/auth/*`) and a gRPC `WhoAmI` RPC used by the gateway.
- The `.proto` for Auth is in `services/auth/proto/auth.proto` (you can generate Go stubs with `protoc`).
