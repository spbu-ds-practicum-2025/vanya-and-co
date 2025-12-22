@echo off
echo Starting all services...
echo.
echo Opening 4 terminal windows:
echo   1. Auth Service (port 5100)
echo   2. File Service (port 5200)
echo   3. Sharing Service (port 5300)
echo   4. Gateway (port 8080)
echo.

start "Auth Service" cmd /k "start-auth.bat"
timeout /t 2 /nobreak >nul

start "File Service" cmd /k "start-file.bat"
timeout /t 2 /nobreak >nul

start "Sharing Service" cmd /k "start-sharing.bat"
timeout /t 2 /nobreak >nul

start "Gateway" cmd /k "start-gateway.bat"

echo.
echo All services started!
echo Open http://localhost:8080 in your browser
echo.
pause