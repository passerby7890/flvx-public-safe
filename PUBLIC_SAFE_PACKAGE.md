# FLVX Public-Safe Package

This folder is a sanitized export intended as a GitHub-ready starting point.

## Included

- Backend source: `go-backend/`
- Agent/gost source: `go-gost/`
- Frontend source: `vite-frontend/`
- Public docs, install scripts, deployment templates, tests, and licenses
- Fresh-VPS source build path: `install-public-safe.sh` and `docker-compose.source.yml`
- Sanitized PostgreSQL package: `database/postgresql/`

## Removed

- Git history and local temporary files
- Production rollout notes from `plans/`
- `node_modules`, frontend build output, coverage/cache folders
- Local backups, packaged binaries, archives, databases, logs, and private env files
- Maintenance-only AI skill content that referenced production hosts
- Generated Windows binaries such as `*.exe`

## Sanitized

- Production hostnames and domains were replaced with `example-*` placeholders.
- Production IPs were replaced with RFC documentation addresses such as `203.0.113.0/24`, `198.51.100.0/24`, and `2001:db8::/32`.
- Real credentials, JWT secrets, database passwords, private keys, and certificates are not included.

## Before Publishing

- Review `README.md`, install scripts, and GitHub workflows for the target repository owner/name.
- Replace placeholder config values in `.env.example` or deployment templates.
- Generate new release artifacts from a clean CI build; do not reuse production binaries.
- Use `install-public-safe.sh` for fresh VPS installs from this public-safe source tree.
- Run the scan commands in the verification section before pushing.

## Verification Performed

```powershell
rg -n "BEGIN .*PRIVATE KEY|real-password|real-jwt-secret|real-panel-host|production-domain" -S
```

Remaining matches after sanitization are limited to placeholder env examples or test PEM markers.

```powershell
go test ./internal/http/handler
go test ./socket -run "TestRenderEntryDemux|TestNormalizeEntryDemux|TestRenderCoverSite"
```
