# Stage 1: Build weather-web server
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod .
RUN go mod download 2>/dev/null || true
COPY main.go .
COPY index.html .
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Stage 2: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy weather-goat CLI binary
COPY bin/weather-goat-pp-cli /app/bin/weather-goat-pp-cli
RUN chmod +x /app/bin/weather-goat-pp-cli

# Copy server binary + index.html
COPY --from=builder /app/server .
COPY --from=builder /app/index.html .

EXPOSE 8097
ENV PORT=8097
ENV CLI_BIN=/app/bin/weather-goat-pp-cli
CMD ["./server"]
