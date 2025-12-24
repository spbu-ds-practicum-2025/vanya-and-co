@echo off
echo Testing service connections...
echo.

echo Testing Auth service (port 5100):
curl -s http://localhost:5100/health || echo "Auth HTTP not responding"

echo Testing File service (port 5201):
curl -s http://localhost:5201/health || echo "File HTTP not responding"

echo Testing Sharing service (port 5400):
curl -s http://localhost:5400/health || echo "Sharing HTTP not responding"

echo Testing Gateway (port 8080):
curl -s http://localhost:8080/health || echo "Gateway not responding"

echo.
echo Checking running processes...
tasklist | findstr "go.exe" || echo "No Go processes found"

pause
