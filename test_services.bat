@echo off
echo Testing services...

echo Starting auth service...
start "Auth" cmd /c "call start-auth.bat"

timeout /t 3 /nobreak >nul

echo Starting file service...
start "File" cmd /c "call start-file.bat"

timeout /t 3 /nobreak >nul

echo Starting sharing service...
start "Sharing" cmd /c "call start-sharing.bat"

timeout /t 3 /nobreak >nul

echo Starting gateway...
start "Gateway" cmd /c "call start-gateway.bat"

echo All services started. Press any key to stop...
pause >nul

echo Stopping services...
taskkill /im go.exe /f >nul 2>&1
echo Done.
