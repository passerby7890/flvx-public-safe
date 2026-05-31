# 安装指南

这份指南说明如何在一台新的 Linux VPS 上安装 public-safe 源码包。

## 系统要求

- Debian 或 Ubuntu 系 Linux VPS。
- root 或 sudo 权限。
- 如需公网访问，请放行面板前端/后端端口。
- 源码构建建议至少 2 GB 内存。

## 快速安装

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
git clone https://github.com/passerby7890/flvx-public-safe.git
cd flvx-public-safe
sudo ./install-public-safe.sh
```

脚本会在 `.env` 不存在时自动创建它，并启动：

- PostgreSQL 16
- 从 `go-backend/` 构建的 FLVX 后端
- 从 `vite-frontend/` 构建的 FLVX 前端
- 从 `go-gost/` 构建并挂载到 `/agent/` 的节点安装资产

如果只想启动面板、不构建节点资产，可以运行：

```bash
sudo BUILD_AGENT_ASSETS=0 ./install-public-safe.sh
```

## 手动安装

```bash
cp .env.example .env
```

首次启动前至少修改这些值：

```bash
JWT_SECRET=replace_with_a_new_random_secret
POSTGRES_PASSWORD=replace_with_strong_password
DATABASE_URL=postgresql://flux_panel:replace_with_strong_password@postgres:5432/flux_panel?sslmode=disable
```

启动服务：

```bash
docker compose -f docker-compose.source.yml up -d --build
```

## 验证安装

```bash
docker compose -f docker-compose.source.yml ps
curl -fsS http://127.0.0.1:${BACKEND_PORT:-6365}/flow/test
```

浏览器打开：

```text
http://<服务器IP>:6366
```

默认账号：

```text
admin_user / admin_user
```

首次登录后请修改密码。

## PostgreSQL 表结构

默认使用 `DB_TYPE=postgres`，后端启动时会自动执行表结构校正。公开安全的 SQL 包也放在：

```text
database/postgresql/
```

只有在需要预创建或审计表结构时才需要使用 `database/postgresql/schema.sql`；普通安装不需要手动导入 SQL。

## 更新或重新构建

拉取新源码后执行：

```bash
docker compose -f docker-compose.source.yml up -d --build
```

## 停止服务

```bash
docker compose -f docker-compose.source.yml down
```

PostgreSQL 数据会保留在 Docker volume：`flvx_postgres_data`。
