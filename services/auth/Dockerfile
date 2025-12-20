FROM golang:1.24 AS builder
WORKDIR /src
COPY ../../go.mod ../../go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/auth ./services/auth/cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/auth /bin/auth
EXPOSE 5100 5101
ENTRYPOINT ["/bin/auth"]
