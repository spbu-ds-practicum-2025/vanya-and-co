#!/bin/bash

# Скрипт для запуска Auth сервиса отдельно

echo "🔐 Starting Auth Service..."
echo "HTTP Port: 5100"
echo "gRPC Port: 5101"
echo ""

# Устанавливаем переменные окружения
export HTTP_PORT=5100
export GRPC_PORT=5101

# Переходим в директорию проекта
cd "$(dirname "$0")"

# Запускаем сервис
go run services/auth/cmd/server/main.go