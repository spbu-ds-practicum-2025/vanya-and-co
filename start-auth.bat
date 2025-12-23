@echo off
echo Starting Auth Service...
echo HTTP Port: 5100
echo gRPC Port: 5101
echo.

set HTTP_PORT=
set GRPC_PORT=
set HTTP_PORT=5100
set GRPC_PORT=5101

go run services\auth\cmd\server\main.go
pause