# Waka Personal

Waka Personal is a self-hosted, single-user WakaTime-compatible backend with a built-in dashboard.
It lets you keep using WakaTime clients and plugins, store the data in PostgreSQL, and browse your own coding activity from a local web UI.

Good fit if you want to:

- keep your coding history in your own database
- point WakaTime-compatible plugins at your own API
- run a personal dashboard on localhost or your own server
- import old backup exports and keep everything in one place

## Highlights

- WakaTime-compatible ingest and query endpoints for heartbeats, durations, summaries, stats, status bar, and file experts
- Built-in Astro + React dashboard, served by the Go app from `dist/`
- PostgreSQL storage with `goose` migrations
- Backup importer for `.json` and `.json.gz` exports
- Optional API key auth for a single-user setup
- Health endpoints, rate limiting, and security headers out of the box

## Stack

- Backend: Go 1.26, Fiber v2, pgx/v5, goose, zerolog
- Frontend: Astro, React, Tailwind CSS
- Database: PostgreSQL

## Project Layout

```text
cmd/             app entrypoints
internal/http/   Fiber handlers and transport concerns
internal/service/ business logic
internal/store/  PostgreSQL access and migrations
src/             Astro + React dashboard
db/migrations/   goose migration files
```

## Quick Start

1. Copy `.env.example` to `.env` and adjust values.
2. Start PostgreSQL:

```shell
docker compose up -d postgres
```

Set `DATABASE_URL` in `.env` to match the Docker database name if you use the bundled Compose file:

```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/dvgamerr?sslmode=disable
```

3. Install frontend dependencies and build the dashboard:

```shell
bun install
bun run build:dist
```

4. Start the API:

```shell
go run ./cmd
```

The API applies pending `goose` migrations on startup.
If `dist/index.html` exists, the Go server also serves the dashboard from the same origin.

## What The API Supports

- `POST /api/v1/users/current/heartbeats`
- `POST /api/v1/users/current/heartbeats.bulk`
- `GET /api/v1/users/current/heartbeats?date=YYYY-MM-DD`
- `DELETE /api/v1/users/current/heartbeats.bulk`
- `GET /api/v1/users/current/durations`
- `GET /api/v1/users/current/summaries`
- `GET /api/v1/users/current/stats`
- `GET /api/v1/users/current/statusbar/today`
- `POST /api/v1/users/current/file_experts`
- `GET /api/v2/dashboard` (dashboard aggregate endpoint, no auth)
- `GET /api/v2/live` (live dashboard slice, no auth, used by the web UI every 60 seconds)
- `GET /api/v2/insights` (insights page data, no auth)
- `GET /api/v2/wrapped` (wrapped page data, no auth)

## Client Config

### VS Code example

```json
{
  "wakatime.apiKey": "your-local-api-key",
  "wakatime.apiUrl": "http://localhost:8080/api/v1"
}
```

### `.wakatime.cfg` example for this project

```ini
[settings]
api_url = http://localhost:8080/api/v1
api_key = <APP_API_KEY>
debug = false
```

### Multi-backend `.wakatime.cfg` example

This is useful when you want to send heartbeats to both Waka Personal and WakaTime (or Wakapi). When `api_key` is set, heartbeats always go to the primary `api_url`, and also to every matching pattern in `[api_urls]`:

```ini
[settings]
api_url = http://localhost:8080/api/v1
api_key = <APP_API_KEY>

debug = false
[api_urls]
.* = https://api.wakatime.com/api/v1|<api_key_waka>
.* = https://wakapi.dev/api|<api_key_wakapi>
```

Each endpoint authenticates with its own key: `api_key` in `[settings]` must be the key for the primary `api_url` (here `APP_API_KEY`), and every entry in `[api_urls]` carries its own key after the `|`. Putting your wakatime.com key in `[settings]` while `api_url` points at Waka Personal makes the server reject every heartbeat, and they silently pile up in the offline queue.

Prefer Waka Personal as the primary `api_url`. The status bar and `wakatime-cli --today` read from the primary endpoint's `/statusbar/today`, and wakatime.com excludes the `AI Coding` category from its status bar total — so with wakatime.com as primary, the status bar can show empty even though heartbeats are syncing fine. Waka Personal counts all categories, so keeping it primary makes the status bar reflect your real total.

Use the same value for `APP_API_KEY` in `.env` and in your client config.
WakaTime plugins send the API key as the Basic-auth username, and this server only checks that the received value matches `APP_API_KEY`.

## Frontend Environment

If the frontend and backend share the same origin, leave `PUBLIC_API_BASE` empty.

Use these variables when building the dashboard:

- `PUBLIC_API_BASE` for the API origin when frontend and backend are on different origins
- `PUBLIC_APP_TIMEZONE` for dashboard queries

The built-in dashboard uses `GET /api/v2/dashboard` for the main aggregate and polls `GET /api/v2/live` every 60 seconds for live widgets. `/insights` and `/wrapped` load from `GET /api/v2/insights` and `GET /api/v2/wrapped`. These web UI endpoints skip API key auth.

## Import Backup JSON

Supports backup files in `.json` and `.json.gz` format with top-level `user`, `range`, and `days[].heartbeats[]`.

```shell
go run ./cmd/importer --file E:\path\to\backup.json
```

Optional flags:

- `--format backup-json`
- `--dry-run`

On Windows, building or running the DuckDB-backed importer from source requires `CGO_ENABLED=1` and a GCC toolchain such as MSYS2 UCRT64.

## Migration Commands

```shell
go run ./cmd/migrate up
go run ./cmd/migrate status
go run ./cmd/migrate down
go run ./cmd/migrate create add_new_table sql
go run ./cmd/migrate fix
go run ./cmd/migrate validate
```

Migration config comes from `.env`, including `DATABASE_URL`, `MIGRATION_DIR`, and `GOOSE_TABLE`.

## Development

```shell
go run ./cmd
go run ./cmd/migrate up
go run ./cmd/importer --file <path-to-backup.json>
go test ./...
bun run build:dist
```

On Windows, avoid rebuilding over the same running `.exe` path. `go build -o bin/waka.exe ./cmd/main.go` will fail while `bin/waka.exe` is still running because Windows locks the file.

Use one of these instead:

```powershell
go run ./cmd
.\scripts\dev-api.ps1
.\scripts\dev-api.ps1 -NoRun
```

`.\scripts\dev-api.ps1` builds a timestamped `bin/waka-dev-*.exe`, removes old stopped dev binaries, and runs the new build without colliding with the previous process.

## Notes

- Auth is bypassed when `APP_API_KEY` is empty, so set it in production.
- If recent activity looks missing, check for heartbeats stuck in the client's offline queue: `wakatime-cli --offline-count`, then flush with `wakatime-cli --sync-offline-activity 100`.
- The default config database name is `waka_personal`, but `docker-compose.yml` uses `dvgamerr`.
- When changing heartbeat persistence or import behavior, check both the live ingest path and the importer path.
