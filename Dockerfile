# Stage 1: Build weather-goat CLI + weather-web server
FROM golang:1.26-alpine AS builder
WORKDIR /app

# Download printing-press-library (weather-goat CLI)
RUN git clone --depth 1 https://github.com/mvanhorn/printing-press-library.git /ppl && \
    cd /ppl/library/other/weather-goat && \
    CGO_ENABLED=0 GOOS=linux go build -o /app/bin/weather-goat-pp-cli ./cmd/weather-goat-pp-cli

# Build weather-web server
COPY go.mod .
RUN go mod download 2>/dev/null || true
COPY main.go .
COPY index.html .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# Stage 2: Runtime
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app

# Copy binaries + HTML from builder
COPY --from=builder /app/bin/weather-goat-pp-cli /app/bin/weather-goat-pp-cli
COPY --from=builder /app/server .
COPY --from=builder /app/index.html .
RUN chmod +x /app/bin/weather-goat-pp-cli

EXPOSE 8097
ENV PORT=8097
ENV CLI_BIN=/app/bin/weather-goat-pp-cli
CMD ["./server"]
