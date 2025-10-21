FROM golang:1.21-bullseye AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o casspaces ./cmd/casspaces

FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ca-certificates sqlite3 && rm -rf /var/lib/apt/lists/*
WORKDIR /root/
COPY --from=builder /app/casspaces .
COPY --from=builder /app/configs/ ./configs/

# Create necessary directories
RUN mkdir -p /var/lib/casspaces /var/log/casspaces /etc/casspaces

EXPOSE 8080
CMD ["./casspaces"]