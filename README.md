# Waka Personal

GoFiber API server with PostgreSQL storage that can act as a single-user WakaTime-compatible backend for `vscode-wakatime`.

## Features

- WakaTime-compatible endpoints for:
  - `POST /api/v1/users/current/heartbeats`
  - `POST /api/v1/users/current/heartbeats.bulk`
  - `GET /api/v1/users/current/heartbeats?date=YYYY-MM-DD`
  - `DELETE /api/v1/users/current/heartbeats.bulk`
  - `GET /api/v1/users/current/statusbar/today`
  - `POST /api/v1/users/current/file_experts`
- PostgreSQL persistence
- Single-user API key auth
- Large backup import CLI using the DuckDB Go client

## Run Locally

1. Copy `.env.example` to `.env` and adjust values.
2. Start PostgreSQL with:

```shell
docker compose up -d postgres
```

3. Run the API:

```shell
go run ./cmd
```

The API and importer automatically run pending `goose` migrations on startup.

## Static Frontend Build

The Astro frontend now builds as a static site. Dashboard data is fetched by the React island in the browser instead of during Astro SSR.

If the frontend and Go API share the same origin, leave `PUBLIC_API_BASE` empty and keep using the `/api` path.

If you serve the static frontend from a different origin, set:

- `PUBLIC_API_BASE` to the API origin, for example `http://localhost:8080`
- `PUBLIC_APP_TIMEZONE` to the timezone the dashboard should request
- `PUBLIC_APP_API_KEY` to the same value as `APP_API_KEY` when API auth is enabled

## Migration Commands

Run migrations explicitly with:

```shell
go run ./cmd/migrate up
```

Common commands:

- `go run ./cmd/migrate status`
- `go run ./cmd/migrate down`
- `go run ./cmd/migrate create add_new_table sql`
- `go run ./cmd/migrate fix`
- `go run ./cmd/migrate validate`

Config comes from `.env`, including `DATABASE_URL`, `MIGRATION_DIR`, and `GOOSE_TABLE`.

## Point VS Code to This API

Set these in VS Code settings:

```json
{
  "wakatime.apiKey": "your-local-api-key",
  "wakatime.apiUrl": "http://localhost:8080/api/v1"
}
```

The extension normalizes the base URL, so use the API root and not a specific heartbeat path.

This server only compares strings for auth, so `APP_API_KEY` only needs to match whatever the client sends.

If a specific client validates API key format on its own, choose a key value that client accepts. That is a client constraint, not a server requirement.

## Import Backup JSON

Supports `.json` and `.json.gz` backup files with top-level `user`, `range`, and `days[].heartbeats[]`.

On Windows, building/running the importer from source with the DuckDB Go client requires `CGO_ENABLED=1` plus a GCC toolchain such as MSYS2 UCRT64.

```shell
go run ./cmd/importer --file E:\path\to\backup.json
```

Optional flags:

- `--format backup-json`
- `--dry-run`

The importer:

1. Reads backup metadata
2. Updates the singleton `import_profile`
3. Creates a new `import_snapshot` record
4. Uses the DuckDB Go client to flatten large JSON into CSV
5. Bulk loads rows into PostgreSQL

## Verification

Run tests:

```shell
go test ./...
```
