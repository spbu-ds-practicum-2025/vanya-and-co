#!/bin/bash

# Скрипт для запуска Sharing сервиса отдельно

echo "🔗 Starting Sharing Service..."
echo "gRPC Port: 5300"
echo ""

# Устанавливаем переменные окружения
export GRPC_PORT=5300

# Переходим в директорию проекта
cd "$(dirname "$0")"

# Запускаем сервис
go run services/sharing/cmd/server/main.go