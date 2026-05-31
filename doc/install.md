# Install

Use the public-safe source-build installer for a fresh VPS:

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
git clone https://github.com/passerby7890/flvx-public-safe.git
cd flvx-public-safe
sudo ./install-public-safe.sh
```

The installer starts PostgreSQL, builds the backend and frontend Docker images
from this repository, generates local secrets, and lets the backend reconcile
the schema at startup.

Manual start:

```bash
cp .env.example .env
docker compose -f docker-compose.source.yml up -d --build
```

Default panel URL:

```text
http://<server-ip>:6366
```

Default login:

```text
admin_user / admin_user
```

Change the password immediately after first login.

For the optional PostgreSQL SQL package, see `database/postgresql/`.
