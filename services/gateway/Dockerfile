FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Копируем статические файлы
RUN mkdir -p /app/static
COPY services/gateway/cmd/server/static/ /app/static/

# Собираем бинарник
WORKDIR /app/services/gateway/cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/gateway-server .

FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Копируем бинарник и статические файлы
COPY --from=builder /app/gateway-server /app/gateway-server
COPY --from=builder /app/static /app/static

EXPOSE 8080

ENTRYPOINT ["/app/gateway-server"]