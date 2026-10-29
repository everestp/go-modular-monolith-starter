# Getting started

This is a beginner-friendly walkthrough for getting the starter running locally.

## 1. Clone and prepare

```bash
git clone <repo-url>
cd prod-go
cp .env.example .env
```

## 2. Start PostgreSQL

Use Docker Compose:

```bash
docker compose -f deploy/docker-compose.yml up -d db
```

## 3. Run the app

```bash
make run
```

The server listens on `:8080` by default.

## 4. Call the app

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/version
```

## 5. Follow a request through the layers

A typical request flows like this:

- `cmd/api/main.go` - starts the process and builds the app
- `internal/app/app.go` - constructs the router and server
- `internal/config/config.go` - loads environment values
- `internal/modules/auth/service.go` - validates business rules
- `internal/modules/users/users.go` - user entity contract
- `internal/platform/security/*.go` - hashing and JWT concern

## 6. Make your first change

Try adding a new endpoint in `internal/app/app.go`, or add a custom validator in a module package. Keep business logic in service code and keep HTTP handling thin.
