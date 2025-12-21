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
RUN cd services/sharing/cmd/server && \
    go build -o /build/sharing-server -ldflags="-s -w" .

# Финальный образ
FROM alpine:3.18

RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Копируем бинарник
COPY --from=builder /build/sharing-server /app/sharing-server

# Создаем директории для данных
RUN mkdir -p /app/services/sharing/data /app/services/file/data

EXPOSE 5300

ENTRYPOINT ["/app/sharing-server"]