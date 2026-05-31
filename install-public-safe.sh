#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

compose_file="docker-compose.source.yml"
if [[ ! -f "$compose_file" ]]; then
  echo "ERROR: run this script from the FLVX repository root." >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is not installed. Installing Docker using get.docker.com..."
  curl -fsSL https://get.docker.com | sh
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "ERROR: Docker Compose v2 is required." >&2
  exit 1
fi

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 48 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 64
  fi
}

if [[ ! -f .env ]]; then
  cp .env.example .env
  jwt_secret="$(random_secret)"
  pg_password="$(random_secret)"
  sed -i "s|replace_with_a_new_random_secret|${jwt_secret}|g" .env
  sed -i "s|replace_with_strong_password|${pg_password}|g" .env
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

if [[ -z "${JWT_SECRET:-}" || "${JWT_SECRET}" == "replace_with_a_new_random_secret" ]]; then
  echo "ERROR: set JWT_SECRET in .env before starting." >&2
  exit 1
fi

if [[ -z "${POSTGRES_PASSWORD:-}" || "${POSTGRES_PASSWORD}" == "replace_with_strong_password" ]]; then
  echo "ERROR: set POSTGRES_PASSWORD in .env before starting." >&2
  exit 1
fi

docker compose -f "$compose_file" up -d --build

echo ""
echo "FLVX is starting."
echo "Frontend: http://<server-ip>:${FRONTEND_PORT:-6366}"
echo "Backend:  http://<server-ip>:${BACKEND_PORT:-6365}"
echo "Default login: admin_user / admin_user"
echo "Change the default password immediately after first login."
