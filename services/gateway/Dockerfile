FROM golang:1.22-alpine AS builder

WORKDIR /build

# Используем прокси для ускорения загрузки
ENV GOPROXY=https://goproxy.io,direct
ENV GO111MODULE=on
ENV CGO_ENABLED=0

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник из корня проекта
RUN cd services/gateway/cmd/server && \
    go build -o /build/gateway-server -ldflags="-s -w" .

# Финальный образ
FROM alpine:3.18

RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Копируем бинарник
COPY --from=builder /build/gateway-server /app/gateway-server

# Копируем статические файлы
COPY services/gateway/cmd/server/static /app/static

EXPOSE 8080

ENTRYPOINT ["/app/gateway-server"]