
# Backend (Golang)
### Stack / Arch

- **Go 1.26 + Fiber v2 + pgx/v5 + goose + zerolog**
- DB = **PostgreSQL** only
- Entry points = **`cmd/`** (`cmd/main.go`, `cmd/migrate`, `cmd/importer`)
- Layers = **`internal/domain` -> `internal/store` -> `internal/service` -> `internal/http`**

### Code Rules (non-negotiable)

- เขียนแบบ **idiomatic Go**: small interfaces, small structs, constructor-style `New...`
- รับ `context.Context` เป็น arg แรกเสมอเมื่อเป็นงาน I/O หรือข้าม layer
- Error handling = **wrap ด้วย `fmt.Errorf(... %w ...)`** ห้ามกลบ root cause
- Logging = **`zerolog` only** ห้าม `fmt.Println` / `log.Println` สำหรับ app logs
- Config ทั้งหมดต้องวิ่งผ่าน **`internal/config`** และ env vars, ห้าม hardcode DSN/port/secret
- ใช้ `go test ./...` เป็น baseline verification ทุกครั้งที่แก้ backend behavior

### Layer Rules

- **`internal/domain`**: pure types / identifiers only, ห้ามรู้เรื่อง HTTP, SQL, Fiber
- **`internal/store`**: SQL, persistence, pgx transactions, row scanning only; ห้ามใส่ HTTP concerns
- **`internal/service`**: business rules, normalization, aggregation, orchestration; ห้ามคืน `fiber.Map` หรือผูกกับ transport
- **`internal/http`**: request parsing, auth, response shape, status codes, middleware; อย่าใส่ business logic ยาว ๆ
- Handler ต้องคุยผ่าน service interfaces, ไม่ยิง SQL ตรงจาก HTTP layer

### DB / Migration Rules

- Migration files อยู่ใน **`db/migrations/`** และใช้ **goose**
- ทุก schema change ต้องมี migration คู่กับ code ที่ใช้ schema นั้น
- Query SQL ให้เก็บใน **`internal/store`** เป็นหลัก ไม่กระจายไปหลาย package
- ถ้าแก้ import flow หรือ persistence shape ต้องเช็คทั้ง API path และ importer path

### Commands

- Run API: **`go run ./cmd`**
- Run migrations: **`go run ./cmd/migrate up`**
- Check migration status: **`go run ./cmd/migrate status`**
- Import backup: **`go run ./cmd/importer --file <path-to-backup.json>`**
- Test all: **`go test ./...`**



# Frontend (Javascript)
### Stack / Arch

- **Bun + Astro (SSR/UI routing) + React (interactive UI, `.jsx`)**
- Router = **Astro**, UI = **React components** in `src/components/`
- State = plain `useState`/`useEffect`, no global state manager; theme via `src/stores/theme.jsx` context

### Code Rules (non-negotiable)

- **Vanilla JS only** (no TS, no JSDoc)
- Format = **Prettier only** (`semi:false`, `singleQuote:true`)
- Prefer `const` + arrow fn (avoid `function`)
- Commit flow: **Husky + lint-staged** (format-on-commit)
- **ห้ามเขียน API logic ใน Astro** (`src/pages/api/`) — API ทั้งหมดต้องเขียนใน **Go backend** (`internal/http/`) เท่านั้น; Astro ทำหน้าที่ SSR/routing เท่านั้น

### UI Rules

- Reuse-first: เช็ค **`src/components/ui/`** ก่อนสร้างใหม่
- Chart helpers อยู่ใน **`src/components/dashboard/dashboardUtils.js`** — ใช้ก่อนสร้างใหม่
- Styling = **Tailwind + theme tokens** (`text-foreground`, `bg-background`) รองรับ **dark/light**
- **Design: No border-radius** — ห้ามใช้ `rounded-*` classes ทั้งหมด (sharp corners only)

### Frontend Env Vars

- `PUBLIC_API_BASE` — API origin (leave empty if frontend + API share same origin)
- `PUBLIC_APP_TIMEZONE` — timezone for dashboard queries
- `PUBLIC_APP_API_KEY` — same as backend `APP_API_KEY` when auth is enabled



# Gotchas

- **Auth is bypassed when `APP_API_KEY` is empty** — always set this in production
- **docker-compose DB name is `dvgamerr`**, but config default is `waka_personal` — set `DATABASE_URL` explicitly
- **DuckDB importer** is behind a build tag (`importer_duckdb.go` + stub `importer_duckdb_stub.go`) — normal builds use the stub
- **Import batches are not auto-cleaned** on failure; retry is safe due to SHA256 deduplication
- **WakaTime plugins send API key as Basic-auth username** — `Authorization: Basic base64(<key>:)`
- When modifying heartbeat schema, check both the API ingest path AND the importer path

See [README.md](README.md) for local setup, Docker commands, and migration usage.
