@echo off
echo Starting Sharing Service...
echo gRPC Port: 5300
echo HTTP Port: 5400
echo Database Path: services\sharing\data\sharing.db
echo.

set GRPC_PORT=5300
set HTTP_PORT=5400
set DB_PATH=%CD%\services\sharing\data\sharing.db

echo DB_PATH: %DB_PATH%
echo.

if not exist services\sharing\data mkdir services\sharing\data

go run services\sharing\cmd\server\main.go