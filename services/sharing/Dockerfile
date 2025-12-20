FROM golang:1.24 AS builder
WORKDIR /src
COPY ../../go.mod ../../go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/sharing ./services/sharing/cmd/server

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/sharing /bin/sharing
EXPOSE 5300
ENTRYPOINT ["/bin/sharing"]
