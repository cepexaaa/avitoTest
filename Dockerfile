FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/server cmd/server/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/server .
COPY --from=builder /app/migrations ./migrations

COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080

CMD ["./server"]
