#!/bin/bash

# Скрипт для запуска File сервиса отдельно

echo "📁 Starting File Service..."
echo "gRPC Port: 5200"
echo ""

# Устанавливаем переменные окружения
export GRPC_PORT=5200

# Переходим в директорию проекта
cd "$(dirname "$0")"

# Создаем директорию для хранения файлов
mkdir -p storage

# Запускаем сервис
go run services/file/cmd/server/main.go