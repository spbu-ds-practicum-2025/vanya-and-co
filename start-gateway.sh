#!/bin/bash

# Скрипт для запуска Gateway отдельно

echo "🌐 Starting Gateway..."
echo "http:localhost:8080"
echo ""

# Устанавливаем переменные окружения
export PORT=8080
export AUTH_GRPC_ADDR=localhost:5101
export AUTH_HTTP_ADDR=localhost:5100
export FILE_ADDR=localhost:5200
export SHARE_ADDR=localhost:5300

# Переходим в директорию проекта
cd "$(dirname "$0")"

# Запускаем gateway
go run services/gateway/cmd/server/main.go