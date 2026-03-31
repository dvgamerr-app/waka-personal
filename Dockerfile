FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/waka-api ./cmd/main.go && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/waka-importer ./cmd/importer

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/waka-api /usr/local/bin/waka-api
COPY --from=builder /out/waka-importer /usr/local/bin/waka-importer
COPY db/migrations ./db/migrations

EXPOSE 8080

CMD ["waka-api"]

