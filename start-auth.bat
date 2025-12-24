@echo off
echo Starting Auth Service...
echo HTTP Port: 5100
echo gRPC Port: 5101
echo Database Path: services\auth\data\auth.db
echo.

set HTTP_PORT=5102
set GRPC_PORT=5103
set DB_PATH=%CD%\services\auth\data\auth.db

echo DB_PATH: %DB_PATH%
echo.

if not exist services\auth\data mkdir services\auth\data

go run services\auth\cmd\server\main.go