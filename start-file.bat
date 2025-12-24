@echo off
echo Starting File Service...
echo gRPC Port: 5200
echo Storage Path: services\file\storage
echo Database Path: services\file\data\file.db
echo.

set GRPC_PORT=5202
set HTTP_PORT=5201
set STORAGE_PATH=%CD%\storage
set DB_PATH=%CD%\services\file\data\file.db

echo STORAGE_PATH: %STORAGE_PATH%
echo DB_PATH: %DB_PATH%
echo.

if not exist services\file\storage mkdir services\file\storage
if not exist services\file\data mkdir services\file\data

go run services\file\cmd\server\main.go