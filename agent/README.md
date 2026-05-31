# Agent Assets

This directory is mounted to `/usr/share/nginx/html/agent` in `docker-compose-v6.yml`.

Required files for patched-channel upgrades:

- `gost-amd64`
- `gost-amd64.sha256`
- `gost-arm64`
- `gost-arm64.sha256`

Quick sync from local release artifacts:

```bash
./deploy/sync-agent-assets.sh ./artifacts ./agent
```

If your panel is behind reverse proxy or multi-domain access, set config key
`agent_download_base_url` in panel settings as an explicit fallback base URL.
