FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /task-queue ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /task-queue /app/task-queue
COPY --from=builder /app/config /app/config 2>/dev/null || true

EXPOSE 8080
EXPOSE 50051

ENTRYPOINT ["/app/task-queue"]
