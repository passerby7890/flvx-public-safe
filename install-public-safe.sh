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

build_agent_assets() {
  if [[ "${BUILD_AGENT_ASSETS:-1}" == "0" ]]; then
    echo "Skipping agent asset build because BUILD_AGENT_ASSETS=0."
    return
  fi

  local asset_dir="${AGENT_ASSET_DIR:-./agent}"
  local source_dir="go-gost"

  if [[ ! -d "$source_dir" ]]; then
    echo "ERROR: missing go-gost source directory." >&2
    exit 1
  fi

  mkdir -p "$asset_dir"
  cp -f install.sh "$asset_dir/install.sh"
  chmod 0644 "$asset_dir/install.sh"

  local needs_build=0
  for f in gost-amd64 gost-amd64.sha256 gost-arm64 gost-arm64.sha256; do
    if [[ ! -s "$asset_dir/$f" ]]; then
      needs_build=1
    fi
  done

  if [[ "${REBUILD_AGENT_ASSETS:-0}" == "1" ]]; then
    needs_build=1
  fi

  if [[ "$needs_build" != "1" ]]; then
    echo "Agent assets already exist in $asset_dir."
    return
  fi

  echo "Building agent assets from go-gost source..."
  local source_abs asset_abs
  source_abs="$(cd "$source_dir" && pwd -P)"
  asset_abs="$(cd "$asset_dir" && pwd -P)"

  docker run --rm \
    -v "${source_abs}:/src" \
    -v "${asset_abs}:/out" \
    -w /src \
    golang:1.24-bookworm \
    sh -ec 'for arch in amd64 arm64; do
      CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -ldflags="-s -w" -o "/out/gost-$arch" .
      cd /out
      sha256sum "gost-$arch" > "gost-$arch.sha256"
      cd /src
    done'

  chmod 0755 "$asset_dir/gost-amd64" "$asset_dir/gost-arm64"
  chmod 0644 "$asset_dir/gost-amd64.sha256" "$asset_dir/gost-arm64.sha256"
}

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

build_agent_assets

docker compose -f "$compose_file" up -d --build

echo ""
echo "FLVX is starting."
echo "Frontend: http://<server-ip>:${FRONTEND_PORT:-6366}"
echo "Backend:  http://<server-ip>:${BACKEND_PORT:-6365}"
echo "Agent assets: ${AGENT_ASSET_DIR:-./agent}"
echo "Default login: admin_user / admin_user"
echo "Change the default password immediately after first login."
