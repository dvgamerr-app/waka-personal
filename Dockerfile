# Stage 1: Build Astro static files
FROM --platform=$BUILDPLATFORM oven/bun:1.3.14 AS frontend-builder

WORKDIR /app

COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

COPY . .
RUN bun run build:dist

# Stage 2: Build Go binaries
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS go-builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/waka-api ./cmd && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/waka-importer ./cmd/importer && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/waka-migrate ./cmd/migrate

# Stage 3: Final image
FROM debian:bookworm-slim

LABEL org.opencontainers.image.source="https://github.com/dvgamerr-app/waka-personal" \
      org.opencontainers.image.description="Self-hosted, single-user WakaTime-compatible backend with a built-in dashboard"

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system --gid 10001 waka && \
    useradd --system --uid 10001 --gid 10001 --home-dir /app --shell /usr/sbin/nologin waka

COPY --from=go-builder /out/waka-api /usr/local/bin/waka-api
COPY --from=go-builder /out/waka-importer /usr/local/bin/waka-importer
COPY --from=go-builder /out/waka-migrate /usr/local/bin/waka-migrate
COPY --from=frontend-builder /app/dist ./dist
COPY db/migrations ./db/migrations

USER 10001:10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["waka-api", "healthcheck"]

CMD ["waka-api"]
