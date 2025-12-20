FROM golang:1.24 AS builder
WORKDIR /src
COPY ../../go.mod ../../go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gateway ./services/gateway/cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/gateway /bin/gateway
EXPOSE 8080
ENTRYPOINT ["/bin/gateway"]
