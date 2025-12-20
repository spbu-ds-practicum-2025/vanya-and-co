@echo off
echo Starting Sharing Service...
echo gRPC Port: 5300
echo.

set GRPC_PORT=5300

go run services\sharing\cmd\server\main.go
pause