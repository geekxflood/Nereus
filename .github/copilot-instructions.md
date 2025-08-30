# Nereus – AI Coding Agent Instructions

Purpose: make you productive fast in this Go codebase by capturing its architecture, workflows, and house rules. Align with .augment/rules/development.md.

## Big picture
- Domain: SNMPv2c trap ingestion → parse/resolve → correlate → store → notify; metrics and health exposed.
- Orchestration in `internal/app/app.go` (see Initialize/Run/Shutdown) wires components in order: logging → metrics → reload → MIB loader/parser → resolver → storage → correlator → HTTP client → notifier → event processor → SNMP listener.
- Data flow: `listener` receives traps → `mib` parser + `resolver` enrich OIDs → `correlator` groups/sets severity → `storage` batches to SQLite → `notifier` posts webhooks (templated) → `metrics` records.

## Key packages (service boundaries)
- `internal/listener`: SNMPv2c UDP trap server with Start/Stop/IsRunning.
- `internal/mib` + `internal/loader`: load/parse MIBs; hot-reload via `internal/reload`.
- `internal/resolver`: OID/name resolution with caching.
- `internal/correlator`: rule engine; see types `RuleCondition`, `RuleAction`, `EventGroup` in `correlator.go`.
- `internal/storage`: SQLite persistence; batch flush, retention; see `Storage.StoreEvent`, `initSchema`.
- `internal/notifier`: webhook delivery; CUE-backed templates in `internal/notifier/templates/` (e.g., `default.cue`).
- `internal/metrics`: Prometheus metrics; configure via `metrics.*`.
- `internal/events`, `internal/client`, `internal/types`: processing pipeline, HTTP client, shared structs.

## Configuration (CUE-first)
- Central schema: `cmd/schemas/config.cue`. Always add/modify here first and keep backward-compat keys (e.g., `server`, `mibs`, legacy logging keys) per comments.
- CLI: `cmd/root.go`, `cmd/generate.go`, `cmd/validate.go`. Examples for webhooks/Slack in `generate.go`.
- Provider: `github.com/geekxflood/common/config`. Many defaults pulled with `GetString/GetInt/GetDuration`.

## Local dev workflows
- Build (inject version/build info):
  - go build -ldflags "-X main.version=dev -X main.buildTime=$(date +%Y%m%d%H%M%S) -X main.commitHash=$(git rev-parse --short HEAD)" -o build/nereus ./main.go
- Test all: go test ./...
- Lint/security (if installed): golangci-lint run; gosec ./...
- Run CLI:
  - nereus                       # start listener
  - nereus generate --output config.yaml
  - nereus validate --config config.yaml
- Note: `go.mod` replaces `github.com/geekxflood/common => ../common`. Ensure the sibling module exists or remove/adjust for CI.

## Project-specific conventions
- SNMP: v2c only. Validate community; handle malformed packets gracefully; extract trap OID from varbind[1] (see `storage.packetToEvent`).
- Lifecycle: every component supports Start/Stop and health. Use DI; no globals. Pass contexts; log with `github.com/geekxflood/common/logging` and include `component` key.
- Storage: SQLite by default; batching via `BatchSize`/`FlushInterval`. Dedup hash format is `sourceIP:trapOID:community`.
- Templates: notifications use embedded CUE templates; pick by name via config (see README “CUE-Based Template System”).
- Metrics/Health: endpoints and names documented in `docs/METRICS.md` (e.g., `nereus_webhooks_*`, `nereus_traps_*`). Expose on separate port.

## Safe change patterns (examples)
- Add config: update CUE schema (with defaults), load via `config.Provider`, thread through constructors, and document in README/examples.
- Add a metric: register/update in `internal/metrics`, document in `docs/METRICS.md`.
- New feature: prefer new `internal/<feature>` package; define an interface and inject from `app.Initialize`.

## Gotchas
- CGO: `github.com/mattn/go-sqlite3` needs CGO enabled for some targets; cross-compiles may require alternatives.
- Hot reload: register reloadable components with `reload.Manager`; validate new config before applying.

Sources to study: `.augment/rules/development.md`, `internal/app/app.go`, `cmd/schemas/config.cue`, `internal/notifier/templates/`, `docs/METRICS.md`, `README.md`.

If anything above is unclear or missing (e.g., exact listener internals or notifier filters), say what you need and I’ll refine this file.
