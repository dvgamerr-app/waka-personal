# Stage 1: Build Astro static files
FROM oven/bun:latest AS frontend-builder

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .
RUN bun run build:dist

# Stage 2: Build Go binaries
FROM golang:1.26 AS go-builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/waka-api ./cmd/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/waka-importer ./cmd/importer

# Stage 3: Final image
FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

COPY --from=go-builder /out/waka-api /usr/local/bin/waka-api
COPY --from=go-builder /out/waka-importer /usr/local/bin/waka-importer
COPY --from=frontend-builder /app/dist ./dist
COPY db/migrations ./db/migrations

EXPOSE 8080

CMD ["waka-api"]

