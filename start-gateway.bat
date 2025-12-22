@echo off
echo Starting Gateway...
echo Port: 8080
echo.

set PORT=8080
set AUTH_HTTP_ADDR=localhost:5100
set FILE_ADDR=localhost:5200
set SHARE_ADDR=localhost:5300

go run services\gateway\cmd\server\main.go
pause