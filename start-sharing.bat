@echo off
echo Starting Sharing Service...
echo gRPC Port: 5300
echo HTTP Port: 5400
echo.

set GRPC_PORT=
set HTTP_PORT=
set GRPC_PORT=5300
set HTTP_PORT=5400

go run services\sharing\cmd\server\main.go
pause