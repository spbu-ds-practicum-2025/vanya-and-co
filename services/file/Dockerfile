FROM golang:1.24 AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник
WORKDIR /app/services/file/cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/file-server .

FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/file-server /app/file-server

# Создаем директории для данных
RUN mkdir -p /app/storage /app/services/file/data

EXPOSE 5200

ENTRYPOINT ["/app/file-server"]