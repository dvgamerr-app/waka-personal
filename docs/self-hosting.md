# Self-hosting guide

Everything beyond the [quick start](../README.md#deploy-in-5-minutes): configuration, HTTPS,
databases, upgrades, backups and troubleshooting.

## Requirements

- Docker Engine with the Compose plugin (`docker compose version`)
- A `linux/amd64` or `linux/arm64` host (Raspberry Pi 4/5, Apple Silicon and most VPS plans work)

## Provider-side setup

None. Waka Personal speaks the WakaTime API, so the only client change is `api_url` and
`api_key` in `~/.wakatime.cfg` or your editor's WakaTime settings.

## Configuration

Set these in `.env` next to `docker-compose.yml`.

### Required

| Variable            | Purpose                                                                                               |
| :------------------ | :---------------------------------------------------------------------------------------------------- |
| `APP_API_KEY`       | The key your WakaTime plugins send. Generate with `openssl rand -hex 24`. An empty key disables auth. |
| `POSTGRES_PASSWORD` | Password for the bundled PostgreSQL.                                                                  |

### Optional

| Variable        | Default         | Purpose                                                                  |
| :-------------- | :-------------- | :----------------------------------------------------------------------- |
| `APP_TIMEZONE`  | `UTC`           | Timezone for daily totals and the dashboard, for example `Asia/Bangkok`. |
| `PORT`          | `8080`          | Host port for the dashboard and API.                                     |
| `IMAGE_TAG`     | `latest`        | Image tag, for example a release such as `v2.3`.                         |
| `LOG_LEVEL`     | `info`          | `debug`, `info`, `warn` or `error`.                                      |
| `POSTGRES_DB`   | `waka_personal` | Database name of the bundled PostgreSQL.                                 |
| `POSTGRES_PORT` | `5432`          | Host port of PostgreSQL, bound to `127.0.0.1` only.                      |

The app also reads `KEYSTROKE_TIMEOUT_MINUTES` (default `15`), `WRITES_ONLY` (default `false`),
`CORS_ALLOW_ORIGINS` (default `*`) and the `PROFILE_*` variables used by the WakaTime
`/users/current` response. Add them under `environment:` in `docker-compose.yml` if you need them.

## Run

```shell
docker compose up -d        # start
docker compose ps           # both services should be "healthy"
docker compose logs -f app  # follow logs
docker compose down         # stop, keeping data
```

The compose file sets `AUTO_MIGRATE=true`, so the schema is created and upgraded on start.

## HTTPS

WakaTime plugins send the API key in a Basic-auth header, so use HTTPS whenever the server is
reachable outside your machine or LAN.

| Option                                  | When to use                                                                                    |
| :-------------------------------------- | :--------------------------------------------------------------------------------------------- |
| Caddy                                   | Simplest automatic Let's Encrypt: `caddy reverse-proxy --from YOUR_DOMAIN --to localhost:8080` |
| Existing reverse proxy (nginx, Traefik) | Forward `YOUR_DOMAIN` to `http://localhost:8080`.                                              |
| Cloudflare Tunnel or Tailscale          | No open ports; good for a home server.                                                         |

Then use `https://YOUR_DOMAIN/api/v1` as `api_url`.

## Database options

- **Bundled PostgreSQL (default).** Data lives in the `pgdata` volume, published on
  `127.0.0.1` only.
- **External PostgreSQL.** Run the image alone and point it at your server:

  ```shell
  docker run -d --name waka-personal -p 8080:8080 \
    -e APP_API_KEY=REPLACE_ME \
    -e AUTO_MIGRATE=true \
    -e DATABASE_URL='postgres://USER:PASSWORD@DB_HOST:5432/waka_personal?sslmode=require' \
    ghcr.io/dvgamerr-app/waka-personal:latest
  ```

  To apply migrations yourself instead, leave `AUTO_MIGRATE` unset and run
  `docker run --rm -e DATABASE_URL=... ghcr.io/dvgamerr-app/waka-personal:latest waka-migrate up`.

## Upgrade

```shell
docker compose pull
docker compose up -d
```

Pending migrations run on start. Pin `IMAGE_TAG` to a release if you prefer upgrading on your
own schedule. Take a backup first.

## Backup and restore

```shell
docker compose exec -T postgres pg_dump -U postgres -Fc waka_personal > waka-personal.dump
docker compose exec -T postgres pg_restore -U postgres -d waka_personal --clean < waka-personal.dump
```

## Import an old WakaTime export

The importer uses DuckDB, which needs CGO, so the binary in the image cannot import. Run it from
source against the bundled database instead (it applies migrations first):

```shell
git clone https://github.com/dvgamerr-app/waka-personal.git && cd waka-personal
DATABASE_URL='postgres://postgres:REPLACE_ME@127.0.0.1:5432/waka_personal?sslmode=disable' \
  CGO_ENABLED=1 go run ./cmd/importer --file /path/to/backup.json.gz
```

Add `--dry-run` to validate the file first. On Windows, CGO needs a GCC toolchain such as MSYS2
UCRT64. Re-importing the same file is safe: batches are deduplicated by SHA-256.

## Scheduled jobs

None are required. The dashboard computes everything on request.

## Troubleshooting

| Symptom or error                                                      | Fix                                                                                                                                                          |
| :-------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `required variable APP_API_KEY is missing a value`                    | Set `APP_API_KEY` in `.env`; the compose file refuses to start without it.                                                                                   |
| `required variable POSTGRES_PASSWORD is missing a value`              | Set `POSTGRES_PASSWORD` in `.env`.                                                                                                                           |
| `dependency failed to start: container ... postgres ... is unhealthy` | Run `docker compose logs postgres`. A volume created by an older compose file at `/var/lib/postgresql/data` must be migrated with `pg_upgrade` or recreated. |
| `relation "..." does not exist`                                       | Migrations did not run. Set `AUTO_MIGRATE=true` or run `waka-migrate up`.                                                                                    |
| Plugins report `401` or heartbeats pile up offline                    | `api_key` in the client must equal `APP_API_KEY`. Check with `wakatime-cli --offline-count` and flush with `wakatime-cli --sync-offline-activity 100`.       |
| Status bar empty but dashboard has data                               | Make Waka Personal the primary `api_url`; see the multi-backend example in the [development guide](development.md#multi-backend-wakatimecfg-example).        |
| Wrong day boundaries                                                  | Set `APP_TIMEZONE` to your IANA timezone and restart.                                                                                                        |
| Port `8080` already in use                                            | Set `PORT` in `.env`, for example `PORT=8090`.                                                                                                               |
