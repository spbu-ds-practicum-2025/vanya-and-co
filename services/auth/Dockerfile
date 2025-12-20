FROM golang:1.24 AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник
WORKDIR /app/services/auth/cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/auth-server .

FROM alpine:3.18

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/auth-server /app/auth-server

# Создаем директорию для данных
RUN mkdir -p /app/services/auth/data

EXPOSE 5100 5101

ENTRYPOINT ["/app/auth-server"]