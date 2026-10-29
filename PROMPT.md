# MASTER PROMPT: Production-Grade Go Modular Monolith Starter

> Paste everything below this line into Claude Code (or any agentic coding tool that can run shell commands, edit files, and run tests).

---

## ROLE

You are a principal Go backend engineer and reviewer. You build boring, secure, observable, well-tested systems. You value clarity over cleverness. You never leave TODOs, stubs, placeholder code, or "left as an exercise" gaps. Everything you deliver compiles, lints clean, and passes tests.

## OBJECTIVE

Build a **production-ready starter repository** for a **modular monolith REST API** in Go with:
authentication, a simple CRUD resource, PostgreSQL with raw SQL, API versioning, structured production logging, a full test suite, CI, Docker, and beginner-friendly documentation.

The result must be a template an intermediate backend developer can clone, understand in under an hour, run in one command, and extend with confidence.

## WORKING RULES (follow strictly)

1. **Work in phases** (Section 14). After each phase run: `go build ./...`, `go vet ./...`, `golangci-lint run`, `go test -race ./...`. Fix every failure before continuing. Print a short phase report (what was built, commands run, results).
2. **Verify, don't assume.** Look up the latest stable Go version at https://go.dev/dl and set it in `go.mod`. Check latest stable versions of every dependency (`go list -m -versions`) and pin them. Never invent APIs. If unsure about a library API, read its docs or source in the module cache.
3. **No placeholders.** No `TODO`, `FIXME`, `panic("not implemented")`, empty functions, or fake data in production paths.
4. **Smallest sufficient design.** Do not add abstractions, patterns, or dependencies that are not justified. Every third-party dependency must be justified in the README.
5. **Explain the why.** Add short doc comments on every exported symbol and on any non-obvious decision. Record major decisions as ADRs.
6. **If a requirement conflicts with another, stop and state the conflict**, choose the safer option, and document it in an ADR.

## 1. HARD CONSTRAINTS

| Area         | Requirement                                                                                                                                                   |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Language     | Latest stable Go. Use the `go` directive and `toolchain` appropriately. Use Go's `tool` directive in `go.mod` (Go 1.24+) for dev tools where supported.       |
| HTTP         | Standard library `net/http` only, with Go 1.22+ `ServeMux` patterns (`"GET /api/v1/notes/{id}"`, `r.PathValue("id")`). **No** gin, echo, fiber, chi, gorilla. |
| Database     | PostgreSQL 16+ via `github.com/jackc/pgx/v5` and `pgxpool`. **Raw SQL only.** No ORM, no query builder, no sqlc, no code generation for queries.              |
| Migrations   | `pressly/goose` (or `golang-migrate`) with versioned up/down SQL, embedded using `embed`.                                                                     |
| Logging      | `log/slog` only. JSON in production, text in development.                                                                                                     |
| Config       | Environment variables only (12-factor). Parsed and validated at startup. Fail fast.                                                                           |
| IDs          | UUID (prefer UUIDv7 if generated in app, or `gen_random_uuid()` in DB).                                                                                       |
| Time         | Always UTC. `timestamptz` in DB. Inject a `Clock` interface for testability.                                                                                  |
| Dependencies | Keep the list minimal. Standard library first.                                                                                                                |

## 2. ARCHITECTURE

**Modular monolith + layered (clean) architecture + SOLID.**

### Modules

- `auth`: register, login, refresh (rotating), logout, me
- `users`: user entity and persistence
- `notes`: CRUD resource owned by a user

Each module is self-contained and could be extracted into its own service later.

### Layers inside each module

```
modules/<name>/
  module.go            # public surface: constructor + exported interfaces only
  domain/              # entities, value objects, domain errors, repository interfaces
  service/             # use cases / business rules (depends on interfaces only)
  transport/http/      # handlers, DTOs, validation, error mapping, route registration
  repository/postgres/ # raw SQL implementations
```

### Dependency rules (enforce with tooling, not just words)

- Direction: `transport -> service -> domain <- repository`. `domain` imports nothing from the project.
- **No module imports another module's internals.** Cross-module calls go through a small interface exported from the other module's root package (example: `users.Reader` consumed by `auth`), wired in the composition root.
- No HTTP types (`http.Request`, status codes) in `service` or `domain`.
- No SQL outside `repository/postgres`.
- No business logic in handlers.
- Use `internal/` and a **depguard** (or equivalent) lint rule to make violations fail CI. Add an **architecture test** (`go list`/AST-based test) that fails if boundaries are crossed.

### Design principles to apply

- **S**: one reason to change per package/type. **O**: extend via new implementations, not edits. **L**: fakes must honor interface contracts. **I**: small, consumer-defined interfaces. **D**: depend on abstractions; constructor injection.
- Accept interfaces, return concrete structs. Define interfaces where they are consumed.
- No globals, no `init()` side effects, no singletons. One **composition root** in `cmd/api/main.go` + `internal/app`.
- `context.Context` is the first parameter of every I/O function and is propagated to every DB call.
- Transactions through a small `TxManager` abstraction (`WithinTx(ctx, func(ctx) error) error`) so services stay DB-agnostic.

## 3. REPOSITORY LAYOUT

```
.
├── cmd/api/main.go                  # composition root only (thin)
├── internal/
│   ├── app/                         # wires modules, builds router, mounts versions
│   ├── config/                      # env parsing + validation
│   ├── platform/                    # shared infrastructure, zero business logic
│   │   ├── httpx/                   # middleware, JSON helpers, error responder, pagination
│   │   ├── database/                # pgxpool, TxManager, migration runner, pg error mapping
│   │   ├── logger/                  # slog setup, context-aware handler
│   │   ├── security/                # argon2id hasher, JWT manager, token hashing
│   │   ├── clock/                   # Clock interface + real/fake
│   │   └── validator/               # input validation helpers
│   └── modules/
│       ├── auth/
│       ├── users/
│       └── notes/
├── migrations/                      # embedded SQL (NNNN_name.up.sql / .down.sql)
├── api/openapi.yaml                 # OpenAPI 3.1, must match implementation
├── deploy/
│   ├── Dockerfile
│   └── docker-compose.yml
├── docs/
│   ├── adr/                         # architecture decision records
│   ├── ARCHITECTURE.md
│   └── GETTING_STARTED.md           # beginner-friendly walkthrough
├── .github/workflows/ci.yml
├── Makefile
├── .golangci.yml
├── .env.example
├── .gitignore  .dockerignore  .editorconfig
├── LICENSE  SECURITY.md  CONTRIBUTING.md
└── README.md
```

## 4. API DESIGN

### Versioning

- All business routes under `/api/v1`.
- Build routing so a new version is added by mounting a new router (`/api/v2`) **without editing v1**. Each version has its own DTOs and handlers; services are shared.
- Support `Deprecation` and `Sunset` response headers via a reusable middleware. Document the deprecation policy in README.

### Endpoints

| Method | Path                    | Auth     | Notes                                             |
| ------ | ----------------------- | -------- | ------------------------------------------------- |
| POST   | `/api/v1/auth/register` | no       | 201, 409 on duplicate email                       |
| POST   | `/api/v1/auth/login`    | no       | timing-safe, generic error                        |
| POST   | `/api/v1/auth/refresh`  | no       | rotates refresh token                             |
| POST   | `/api/v1/auth/logout`   | yes      | revokes refresh token family                      |
| GET    | `/api/v1/auth/me`       | yes      | current user                                      |
| POST   | `/api/v1/notes`         | yes      | 201 + `Location` header                           |
| GET    | `/api/v1/notes`         | yes      | paginated, filtered, sorted                       |
| GET    | `/api/v1/notes/{id}`    | yes      | owner only                                        |
| PUT    | `/api/v1/notes/{id}`    | yes      | full replace                                      |
| PATCH  | `/api/v1/notes/{id}`    | yes      | partial update                                    |
| DELETE | `/api/v1/notes/{id}`    | yes      | 204, owner only                                   |
| GET    | `/healthz`              | no       | liveness (no dependencies)                        |
| GET    | `/readyz`               | no       | readiness (pings DB with short timeout)           |
| GET    | `/version`              | no       | version, commit, build time                       |
| GET    | `/metrics`              | internal | Prometheus format (optionally on a separate port) |

### Conventions

- Pagination: cursor-based (keyset) preferred; max page size enforced (e.g. 100), default 20. Sorting by a **whitelist** of columns only.
- Errors: **RFC 9457 Problem Details** (`application/problem+json`) with `type`, `title`, `status`, `detail`, `instance`, `request_id`, and field-level `errors` for validation. Never leak internals, SQL, or stack traces.
- Correct status codes: 200, 201, 204, 400, 401, 403, 404, 405, 409, 413, 415, 422, 429, 500, 503.
- Custom JSON 404 and 405 responses (ServeMux default is plain text).
- Return `404` (not `403`) for another user's note to avoid resource enumeration.
- Optional `Idempotency-Key` support on POST create endpoints (documented; implement if time allows, otherwise document the seam).
- `api/openapi.yaml` must be complete and kept in sync; add a test or CI step that validates the spec.

## 5. AUTHENTICATION AND SECURITY

### Credentials and tokens

- Passwords hashed with **argon2id** (parameters from config, sensible defaults per OWASP), constant-time compare, optional pepper from env. Enforce password policy (min length 12, max 128).
- **Access token**: short-lived JWT (default 15 min), signed with HS256 or EdDSA, with `iss`, `aud`, `sub`, `exp`, `iat`, `jti`. Validate all claims and reject `alg=none`. Secret minimum 32 bytes, validated at startup.
- **Refresh token**: opaque random (>= 256 bits), stored **hashed (SHA-256)** in DB, rotated on every use, grouped in a **token family**. **Reuse detection**: if a rotated token is replayed, revoke the whole family. Logout revokes the family. Expired tokens cleaned by a documented cleanup query/job.
- Login must be **timing-safe**: run a dummy hash when the user doesn't exist; return the same generic error for wrong email or password.
- Account enumeration protection on register/login responses where practical.

### HTTP hardening

- Middleware order (outermost to innermost): request ID -> panic recovery -> access logging -> security headers -> CORS (config-driven allowlist) -> rate limit -> request timeout -> body size limit -> auth (per route group).
- `http.MaxBytesReader` on bodies; `Content-Type` enforcement; strict JSON decoding with `DisallowUnknownFields` and rejection of trailing data.
- Security headers: `X-Content-Type-Options`, `X-Frame-Options`/CSP frame-ancestors, `Referrer-Policy`, `Cache-Control: no-store` on auth responses, `Strict-Transport-Security` when behind TLS.
- Rate limiting: in-memory token bucket per IP, behind a `RateLimiter` interface so Redis can replace it. Stricter limits on auth endpoints. Correct client IP handling with configurable trusted proxies (never trust `X-Forwarded-For` blindly).
- Parameterized queries only. Emails normalized to lowercase (use `citext` or lowercased column) with a unique index.
- Authorization: ownership checks live in the **service layer**, enforced with tests.
- Secrets only from env; never logged; `.env` is gitignored; no secrets in the Dockerfile or image layers.
- Run `gosec` and `govulncheck` in CI.

## 6. DATABASE

- Tables: `users`, `refresh_tokens`, `notes`. UUID PKs, proper FKs with `ON DELETE` behavior chosen deliberately, `created_at`/`updated_at` (`timestamptz`), unique constraints, `CHECK` constraints, and indexes designed for the actual queries (e.g. `(owner_id, created_at DESC, id)` for keyset pagination). Add an `updated_at` trigger or set it explicitly in SQL (document the choice).
- Soft delete on notes (`deleted_at`) with partial indexes, or hard delete; pick one and write an ADR.
- Optimistic concurrency for updates (`version` column or `updated_at` check) and return `409`/`412` on conflict.
- Pool config from env: max/min conns, max conn lifetime, idle time, health check period, connect timeout. Set `statement_timeout` and `application_name`.
- Map pg errors (`23505` unique, `23503` FK, `23514` check, `40001/40P01` serialization/deadlock) to domain errors in one place.
- SQL stored as named constants next to each repository method, or as embedded `.sql` files. Every query is `EXPLAIN`-reasonable and uses explicit column lists (no `SELECT *`).
- Migrations run via a dedicated command (`cmd/migrate` or `api migrate` subcommand), **not** automatically on every app start in production (allow an opt-in flag for dev).
- Dev-only seed script, clearly separated.

## 7. OBSERVABILITY

- `slog` with context-aware handler. Every request log line includes: `request_id`, `method`, `path` (route pattern, not raw path with IDs, to keep cardinality low), `status`, `latency_ms`, `bytes`, `remote_ip`, `user_id` (when authenticated), `user_agent`. Log level by status (5xx error, 4xx warn, else info). Never log bodies, passwords, tokens, or authorization headers.
- Request ID: accept inbound `X-Request-ID` (validated, length-limited) or generate; return it in the response and in every error body.
- Prometheus metrics (stdlib-friendly, minimal dependency): request count, latency histogram by route pattern/method/status class, in-flight gauge, DB pool stats, build info. Document how to scrape.
- Leave a clean seam for OpenTelemetry (middleware + DB tracer hook points documented in `docs/ARCHITECTURE.md`). Do not over-build it.
- `/healthz` must not touch the DB. `/readyz` must, with a 1-2 second timeout.
- Build info (`version`, `commit`, `build_time`) injected with `-ldflags` and logged at startup.

## 8. RUNTIME AND PRODUCTION READINESS

- `http.Server` with explicit `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`, and `BaseContext`.
- **Graceful shutdown** on SIGINT/SIGTERM using `signal.NotifyContext`: stop accepting, fail readiness first, drain in-flight requests for a configurable period, close the DB pool, flush logs. Exit non-zero on startup failure.
- `GOMAXPROCS`/memory awareness in containers (document; use `automaxprocs` only if justified).
- **Dockerfile**: multi-stage, pinned base image digests or versions, `CGO_ENABLED=0`, `-trimpath`, stripped binary, distroless or minimal non-root runtime, read-only root filesystem compatible, `HEALTHCHECK` (or documented probe config), `.dockerignore`.
- **docker-compose.yml**: Postgres (with healthcheck, volume), migration job that runs before the API, API depending on healthy DB, env from `.env`.
- **Makefile** targets (self-documenting `make help`): `run build test test-unit test-integration test-e2e lint fmt vet vuln migrate-up migrate-down migrate-status migrate-create docker-build docker-up docker-down cover tidy tools clean ci`.
- **CI (GitHub Actions)**: jobs for lint, vet, `govulncheck`, unit tests with `-race -shuffle=on`, integration tests with a Postgres service, coverage upload/summary with a minimum threshold, OpenAPI validation, Docker build (+ optional image scan with Trivy), Go module cache. Pin actions by version or SHA. Add Dependabot config.
- Compose a `.golangci.yml` enabling at least: `govet, staticcheck, errcheck, gosec, revive, depguard, errorlint, gocritic, unconvert, unparam, misspell, bodyclose, noctx, sqlclosecheck, rowserrcheck, contextcheck, nilerr, exhaustive, gofumpt`.

## 9. TESTING (mandatory and comprehensive)

| Level                    | What                                                                                                                                                      | How                                                                                                                                                    |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Unit                     | Services, validators, security helpers, pagination                                                                                                        | Table-driven, hand-written fakes (no heavy mocking frameworks), `t.Parallel()` where safe, fake `Clock`                                                |
| Handler                  | Every endpoint: success, validation failure, bad JSON, wrong content type, oversized body, unauthenticated, forbidden/ownership, not found                | `httptest`, real router, fake services or fake repos                                                                                                   |
| Middleware               | Request ID, recovery, logging fields, CORS, rate limit, timeout, auth, security headers                                                                   | Isolated tests                                                                                                                                         |
| Repository / integration | Every SQL query, constraints, error mapping, pagination ordering, concurrency conflict                                                                    | Real Postgres via `testcontainers-go`, migrations applied first, isolation per test (transaction rollback or truncate), behind build tag `integration` |
| End-to-end               | register -> login -> me -> create note -> list -> get -> update -> patch -> delete -> refresh -> **replay old refresh (expect family revoked)** -> logout | Full app wired against real Postgres                                                                                                                   |
| Architecture             | Module boundary and layer rules                                                                                                                           | Test that fails on forbidden imports                                                                                                                   |
| Contract                 | Responses match `openapi.yaml`; error format golden tests                                                                                                 | Spec validation + golden files                                                                                                                         |
| Config                   | Valid, missing, malformed env cases                                                                                                                       | Table-driven                                                                                                                                           |

Requirements:

- `go test -race -shuffle=on ./...` must pass; flaky tests are not acceptable.
- Coverage target: **>= 80% overall, >= 90% on service and security packages**. `make cover` prints a per-package summary and fails below threshold.
- Optional: add a Go native fuzz test for the JSON decoder/validator and a benchmark for password hashing parameters.
- Provide test builders/helpers in a `testutil` package; no test depends on execution order or wall-clock time.

## 10. CODE QUALITY STANDARDS

- Idiomatic Go: errors wrapped with `%w`, `errors.Is/As`, typed and sentinel domain errors, no panics for control flow, no `interface{}`/`any` where a type works, no naked returns in long functions, small functions, meaningful names, no stutter (`notes.Note`, not `notes.NotesNote`).
- Handlers follow one pattern: decode -> validate -> call service -> map result/error -> respond.
- A single error-mapping function translates domain errors to HTTP problem responses.
- Reuse generic JSON helpers (`Decode`, `Respond`, `RespondError`) in `platform/httpx`.
- `gofumpt` + `goimports` formatted. Doc comments on all exported identifiers. Package comments on every package.
- Conventional Commits recommended in CONTRIBUTING.

## 11. CONFIGURATION (env table must be in README)

At minimum: `APP_ENV`, `HTTP_ADDR`, `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT`, `SHUTDOWN_TIMEOUT`, `LOG_LEVEL`, `LOG_FORMAT`, `DATABASE_URL`, `DB_MAX_CONNS`, `DB_MIN_CONNS`, `DB_MAX_CONN_LIFETIME`, `JWT_SECRET`, `JWT_ISSUER`, `JWT_AUDIENCE`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `ARGON2_*`, `PASSWORD_PEPPER`, `CORS_ALLOWED_ORIGINS`, `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`, `TRUSTED_PROXIES`, `METRICS_ADDR`.
Validate types, ranges, and cross-field rules at startup, and print a redacted config summary on boot.

## 12. DOCUMENTATION DELIVERABLES (written for beginners and experts)

Write in plain, friendly language. Assume the reader knows basic Go but not this codebase.

1. **README.md**: what it is, features, quick start (3 commands), architecture diagram (Mermaid), folder map with one-line explanations, env var table, curl examples for every endpoint, how to run tests, Makefile reference, production checklist, FAQ, dependency justification table, known limitations.
2. **docs/GETTING_STARTED.md**: step-by-step first-hour guide: clone, run, register a user, call an endpoint, follow one request through every layer (with file names), make your first change.
3. **docs/ARCHITECTURE.md**: layers, module boundaries, dependency rules, request lifecycle diagram, auth/token flow sequence diagram (Mermaid), error handling flow, transaction handling, observability seams.
4. **How-to recipes** (in docs or README): add a new module, add a new endpoint, add a migration, add API v2, swap the rate limiter for Redis, add a background job.
5. **docs/adr/**: ADR-0001 stdlib router; ADR-0002 raw SQL with pgx; ADR-0003 modular monolith; ADR-0004 token strategy (access + rotating refresh); ADR-0005 error format (RFC 9457); ADR-0006 pagination strategy; ADR-0007 soft vs hard delete. Use Context / Decision / Consequences format.
6. **SECURITY.md** (reporting process, supported versions), **CONTRIBUTING.md**, **LICENSE** (MIT unless told otherwise).
7. **Production checklist** in README: TLS termination, secrets management, DB backups, migrations strategy, connection limits, log shipping, metrics and alerts, rate limit tuning, CORS, dependency updates, rollback plan.

## 13. DEFINITION OF DONE (verify and print results at the end)

- [ ] `make ci` passes locally (fmt, vet, lint, vuln, tests with race, coverage threshold)
- [ ] `docker compose up` brings the stack up and the e2e flow works with curl
- [ ] All endpoints documented in `openapi.yaml` and README with working examples
- [ ] Architecture test proves no module boundary violations
- [ ] No secrets in repo; `.env.example` complete
- [ ] Graceful shutdown demonstrated (send SIGTERM during an in-flight request)
- [ ] Refresh token reuse detection proven by an automated test
- [ ] Ownership isolation proven by an automated test (user A cannot read/modify user B's notes)
- [ ] Zero `TODO`/`FIXME`, zero skipped tests without a documented reason
- [ ] README quick start verified from a clean clone

## 14. EXECUTION PLAN (phases with gates)

**Phase 1: Foundation.** Module init, layout, config, logger, clock, DB pool, TxManager, migrations runner, health/readiness/version endpoints, graceful shutdown, Makefile, lint config, Dockerfile, compose. _Gate: app boots, `/healthz` and `/readyz` work, lint and tests pass._

**Phase 2: Users + Auth.** Migrations, users module, security package (argon2id, JWT, token hashing), auth module (register/login/refresh/logout/me), auth middleware, rotating refresh with reuse detection. _Gate: unit + integration tests green._

**Phase 3: Notes.** Migrations, CRUD with ownership, keyset pagination, sort whitelist, optimistic concurrency, `Location` header. _Gate: handler + repository tests green._

**Phase 4: Hardening.** Full middleware chain, rate limiter interface + implementation, CORS, security headers, RFC 9457 errors, custom 404/405, metrics, deprecation middleware, API v2 seam (mount an empty v2 example in a test). _Gate: middleware tests green._

**Phase 5: Quality and delivery.** E2E tests, architecture test, OpenAPI validation, CI workflow, Dependabot, coverage threshold, all documentation and ADRs, final audit. _Gate: Definition of Done fully checked._

**Final audit pass (mandatory):** Re-read the whole codebase as a hostile reviewer and list findings under: SOLID violations, module boundary leaks, security issues, error-handling gaps, test gaps, performance concerns (N+1, missing indexes), and documentation gaps. **Fix every finding**, re-run all checks, then print:

1. The final directory tree
2. Commands to run, test, and deploy
3. The Definition of Done checklist with real results
4. Trade-offs made and known limitations
5. Suggested next steps (Redis, RBAC, OpenTelemetry, outbox pattern, background jobs)

## OUTPUT STYLE

Be concise in chat, thorough in code and docs. Show command output evidence for claims like "tests pass". Do not ask for permission between phases unless a decision is genuinely ambiguous; make the safest reasonable choice and record it in an ADR.

**Begin with Phase 1 now.**
