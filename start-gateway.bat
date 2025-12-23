@echo off

REM Скрипт для запуска Gateway отдельно

echo  Starting Gateway...
echo http://localhost:8080
echo.

REM Устанавливаем переменные окружения
set PORT=
set AUTH_GRPC_ADDR=
set AUTH_HTTP_ADDR=
set FILE_ADDR=
set SHARE_ADDR=
set SHARE_HTTP_ADDR=
set PORT=8080
set AUTH_GRPC_ADDR=localhost:5101
set AUTH_HTTP_ADDR=localhost:5100
set FILE_ADDR=localhost:5200
set SHARE_ADDR=localhost:5300
set SHARE_HTTP_ADDR=localhost:5400

REM Запускаем gateway
go run services/gateway/cmd/server/main.go