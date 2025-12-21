FROM golang:1.22-alpine AS builder

WORKDIR /build

# Установка необходимых инструментов для сборки с CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Используем прокси для ускорения загрузки
ENV GOPROXY=https://goproxy.io,direct
ENV GO111MODULE=on
ENV CGO_ENABLED=1

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник из корня проекта
RUN cd services/auth/cmd/server && \
    go build -o /build/auth-server -ldflags="-s -w" .

# Финальный образ
FROM alpine:3.18

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Копируем бинарник
COPY --from=builder /build/auth-server /app/auth-server

# Создаем директорию для данных
RUN mkdir -p /app/services/auth/data

EXPOSE 5100 5101

ENTRYPOINT ["/app/auth-server"]