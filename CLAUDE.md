# Repository guidance

Read `AGENTS.md` before changing this repository; its backend layering and
frontend rules are mandatory.

## Working conventions

- Preserve the Go dependency direction:
  `internal/domain -> internal/store -> internal/service -> internal/http`.
- Keep SQL in `internal/store` and transport response shapes in
  `internal/http`.
- Pass `context.Context` first across I/O boundaries and wrap errors with `%w`.
- Keep dashboard aggregate reads independent and concurrent. The shared
  `runParallel` helper intentionally replaces per-query buffered channels while
  preserving partial results and deterministic error ordering.
- Keep `wrapped` on its single summaries scan; do not reintroduce a redundant
  goroutine or stats scan.
- Keep Astro page config in `src/lib/dashboardConfig.js` and navigation markup
  in `SyntaxNavLinks.astro`.
- Prefer semantic Astro elements and do not add a root wrapper unless CSS,
  scripting, or accessibility behavior consumes it.
- The UI has a strict sharp-corner design: no Tailwind `rounded-*` utilities.
- Format only files in scope; do not run broad rewrites when a focused command
  is sufficient.

## Validation

```shell
gofmt -l cmd internal
go test ./...
go vet ./...
go mod verify
go mod tidy -diff
bun install --frozen-lockfile
bun run check
bun audit
git diff --check
```

`golangci-lint run` is also appropriate when the binary is installed. Race
tests on Windows require a CGO-capable toolchain and GCC.

## Environment and secrets

- `.env.example` is the public template; `.env` must stay ignored.
- Never log or document API keys, database credentials, heartbeat contents, or
  imported backup data.
- Keep `PUBLIC_API_BASE` normalization and timezone fallbacks behaviorally
  consistent across every static page.

## Optimization log

### 2026-08-02

- Recorded a clean `main` baseline at commit `8c71b35` before editing.
- Replaced eleven per-request buffered query channels across dashboard, live,
  and wrapped handlers with one WaitGroup-based parallel runner; dashboard and
  live retain their original concurrency, while wrapped now executes its one
  query directly.
- Centralized repeated error collection and nil map/list normalization without
  changing JSON response shapes, and reused the heartbeat response serializer
  for the single-ingest endpoint.
- Replaced four copies of Astro dashboard environment config with one shared
  module and two copies of navigation-link markup with one wrapper-free Astro
  component.
- Removed the unused `#app` layout wrapper, reducing every generated page by
  one DOM element while leaving React island hydration boundaries untouched.
- Verified the generated output: all four pages contain one fewer element,
  navigation link counts remain unchanged, combined HTML decreased by 515
  bytes, and `_astro` assets decreased by 384 bytes for this build.
- Removed seven `rounded-*` violations, set the global radius token to zero,
  and removed five dark-theme chart variables that duplicated inherited root
  values.
- Added `bun run check` and expanded README maintenance and validation notes.
- Tidied Go module metadata, removing unused `x/mod` and `x/tools` requirements
  plus stale checksums without changing selected runtime dependency versions.
- Passed uncached Go tests, Go vet, Go build, gofmt, module verification/tidy
  checks, frozen Bun install, Prettier, and the four-page Astro production
  build. The race check was unavailable because this Windows toolchain has
  neither CGO nor GCC.
- Baseline dependency audit: 44 advisories (13 high, 28 moderate, 3 low).
  Dependency major upgrades remain separate from this behavior-preserving
  refactor.
