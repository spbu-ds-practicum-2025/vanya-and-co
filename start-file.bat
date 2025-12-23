@echo off
echo Starting File Service...
echo gRPC Port: 5200
echo.

set GRPC_PORT=
set GRPC_PORT=5200

if not exist storage mkdir storage

go run services\file\cmd\server\main.go
pause