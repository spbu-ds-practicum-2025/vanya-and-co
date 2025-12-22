@echo off
echo Starting Auth Service...
echo HTTP Port: 5100
echo.

set HTTP_PORT=5100

go run services\auth\cmd\server\main.go
pause