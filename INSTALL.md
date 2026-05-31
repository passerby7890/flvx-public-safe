# Installation Guide

This guide describes the public-safe install path for a new Linux VPS.

## Requirements

- Debian or Ubuntu style Linux VPS.
- Root or sudo access.
- Open inbound TCP ports for the panel frontend and backend if needed.
- At least 2 GB RAM recommended for source builds.

## Quick Install

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
git clone https://github.com/passerby7890/flvx-public-safe.git
cd flvx-public-safe
sudo ./install-public-safe.sh
```

The script creates `.env` if it does not exist and starts:

- PostgreSQL 16
- FLVX backend built from `go-backend/`
- FLVX frontend built from `vite-frontend/`

## Manual Install

```bash
cp .env.example .env
```

Edit these values before first start:

```bash
JWT_SECRET=replace_with_a_new_random_secret
POSTGRES_PASSWORD=replace_with_strong_password
DATABASE_URL=postgresql://flux_panel:replace_with_strong_password@postgres:5432/flux_panel?sslmode=disable
```

Start the stack:

```bash
docker compose -f docker-compose.source.yml up -d --build
```

## Verify

```bash
docker compose -f docker-compose.source.yml ps
curl -fsS http://127.0.0.1:${BACKEND_PORT:-6365}/flow/test
```

Open:

```text
http://<server-ip>:6366
```

Login:

```text
admin_user / admin_user
```

Change the password after the first login.

## PostgreSQL Schema

The backend starts with `DB_TYPE=postgres` and runs schema reconciliation
automatically. A public-safe SQL package is also included at:

```text
database/postgresql/
```

Use `database/postgresql/schema.sql` only when a database operator needs to
pre-create or review the schema. Normal installs do not require manual SQL.

## Upgrade Or Rebuild

After pulling new source changes:

```bash
docker compose -f docker-compose.source.yml up -d --build
```

## Stop

```bash
docker compose -f docker-compose.source.yml down
```

PostgreSQL data remains in the `flvx_postgres_data` Docker volume.
