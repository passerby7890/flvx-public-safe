# FLVX Public-Safe

FLVX is a traffic forwarding management panel built around a Go admin API,
a React/Vite frontend, a PostgreSQL or SQLite data layer, and a forked GOST v3
forwarding stack.

This repository is a public-safe source package. Production domains, IP
addresses, node secrets, certificates, database files, logs, private
maintenance notes, and other deployment-specific material have been removed or
replaced with placeholders.

## Fresh VPS Install

The recommended public-safe install builds the backend and frontend from this
repository on the VPS. It does not depend on old release assets.

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
git clone https://github.com/passerby7890/flvx-public-safe.git
cd flvx-public-safe
sudo ./install-public-safe.sh
```

Open the panel:

```text
http://<server-ip>:6366
```

Default login:

```text
admin_user / admin_user
```

Change the default password immediately after first login.

## What The Installer Does

- Installs Docker if it is missing.
- Creates `.env` from `.env.example` on first run.
- Generates a random `JWT_SECRET`.
- Generates a random PostgreSQL password.
- Starts PostgreSQL, backend, and frontend with `docker-compose.source.yml`.
- Lets the backend run its normal schema reconciliation at startup.

## Manual Install

```bash
cp .env.example .env
# Edit JWT_SECRET and POSTGRES_PASSWORD in .env.
docker compose -f docker-compose.source.yml up -d --build
```

Check service state:

```bash
docker compose -f docker-compose.source.yml ps
curl -fsS http://127.0.0.1:6365/flow/test
```

## PostgreSQL SQL Package

The sanitized PostgreSQL package is in:

```text
database/postgresql/
```

It contains:

- `schema.sql`: public-safe table and index DDL.
- `seed.sql`: sanitized default first-run seed data only.

The application itself uses GORM models and AutoMigrate as the runtime source of
truth. On a normal fresh VPS install, the backend creates or reconciles the
schema automatically. The SQL package is included for operators who need a
reviewable or pre-created PostgreSQL schema.

## Runtime Layout

- Backend API: `go-backend/`
- Frontend UI: `vite-frontend/`
- Forwarding stack: `go-gost/`
- Agent assets mount point: `agent/`
- PostgreSQL docs and SQL package: `database/postgresql/`
- Source-build compose: `docker-compose.source.yml`

## Ports

Defaults:

- Frontend: `6366`
- Backend: `6365`
- PostgreSQL: internal Docker network only

Change ports in `.env`:

```bash
BACKEND_PORT=6365
FRONTEND_PORT=6366
```

## Data Safety

This package intentionally does not include:

- Production database dumps.
- Real node IP addresses or domains.
- Private keys or certificates.
- Production API tokens.
- Logs, runtime databases, or local backups.
- Private maintenance notes.

Before publishing, run a sensitive-content scan for your own environment names,
domains, IP addresses, and credentials.

## Development

Backend tests:

```bash
cd go-backend
go test ./internal/http/handler
go test ./socket -run "TestRenderEntryDemux|TestNormalizeEntryDemux|TestRenderCoverSite"
```

Frontend build:

```bash
cd vite-frontend
npm install --legacy-peer-deps
npm run build
```

## License

This package retains the upstream Apache License 2.0 licensing materials:

- `LICENSE`
- `LICENSE-APACHE`
- `NOTICE`
