FROM golang:1.22-alpine AS builder

WORKDIR /build

# Используем прокси для ускорения загрузки
ENV GOPROXY=https://goproxy.io,direct
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник из корня проекта
RUN cd services/file/cmd/server && \
    go build -o /build/file-server -ldflags="-s -w" .

# Финальный образ
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Копируем бинарник
COPY --from=builder /build/file-server /app/file-server

# Создаем директории для данных
RUN mkdir -p /app/storage /app/services/file/data

EXPOSE 5200

ENTRYPOINT ["/app/file-server"]