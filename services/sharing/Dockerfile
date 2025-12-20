FROM golang:1.24 AS builder

WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарник
WORKDIR /app/services/sharing/cmd/server
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/sharing-server .

FROM alpine:3.18

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/sharing-server /app/sharing-server

# Создаем директории для данных
RUN mkdir -p /app/services/sharing/data /app/services/file/data

EXPOSE 5300

ENTRYPOINT ["/app/sharing-server"]