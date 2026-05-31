# FLVX Public-Safe

FLVX 是一个流量转发管理面板，包含 Go 后端 API、React/Vite 前端、PostgreSQL/SQLite 数据层，以及基于 GOST v3 的转发组件。

这个仓库是可公开发布的源码包。生产域名、真实 IP、节点密钥、证书、数据库文件、日志、私有维护笔记和部署专属资料都已移除或替换为占位值。

## 新 VPS 一键安装

推荐使用源码构建安装路径。安装过程只依赖当前仓库，不依赖旧 release 产物。

```bash
sudo apt-get update
sudo apt-get install -y git curl ca-certificates
git clone https://github.com/passerby7890/flvx-public-safe.git
cd flvx-public-safe
sudo ./install-public-safe.sh
```

安装完成后打开面板：

```text
http://<服务器IP>:6366
```

默认登录账号：

```text
admin_user / admin_user
```

首次登录后请立刻修改默认密码。

## 安装脚本会做什么

- 如果系统没有 Docker，会自动安装 Docker。
- 首次运行时从 `.env.example` 生成 `.env`。
- 自动生成随机 `JWT_SECRET`。
- 自动生成随机 PostgreSQL 密码，并同步写入 `DATABASE_URL`。
- 从 `go-gost/` 源码构建 `agent/` 下的内建节点安装资产。
- 使用 `docker-compose.source.yml` 构建并启动 PostgreSQL、后端和前端。
- 后端启动时会按 GORM 模型自动建表/补齐表结构。

## 手动安装

```bash
cp .env.example .env
# 编辑 .env 里的 JWT_SECRET 和 POSTGRES_PASSWORD
docker compose -f docker-compose.source.yml up -d --build
```

检查服务：

```bash
docker compose -f docker-compose.source.yml ps
curl -fsS http://127.0.0.1:6365/flow/test
```

## PostgreSQL SQL 包

已脱敏的 PostgreSQL 包位于：

```text
database/postgresql/
```

包含：

- `schema.sql`：公开安全的表结构和索引 DDL。
- `seed.sql`：只包含首次运行需要的脱敏种子数据。

正常安装不需要手动导入 SQL；后端会在启动时自动建表和校正结构。SQL 包主要给需要审计或预创建数据库的运维人员使用。

## 目录结构

- `go-backend/`：后端 API。
- `vite-frontend/`：前端面板。
- `go-gost/`：转发组件源码。
- `agent/`：Agent 资源挂载目录。
- `database/postgresql/`：PostgreSQL 文档和 SQL 包。
- `docker-compose.source.yml`：源码构建版 compose。

## Agent 资产

安装脚本默认会生成：

```text
agent/install.sh
agent/gost-amd64
agent/gost-amd64.sha256
agent/gost-arm64
agent/gost-arm64.sha256
```

这些文件会由前端容器通过 `/agent/` 路径提供给节点安装和内建版升级使用。若只想启动面板、不构建节点资产，可以这样运行：

```bash
sudo BUILD_AGENT_ASSETS=0 ./install-public-safe.sh
```

## 默认端口

- 前端：`6366`
- 后端：`6365`
- PostgreSQL：仅在 Docker 内部网络暴露

需要改端口时编辑 `.env`：

```bash
BACKEND_PORT=6365
FRONTEND_PORT=6366
```

## 安全说明

这个公开包不会包含：

- 生产数据库 dump。
- 真实节点 IP 或生产域名。
- 私钥、证书或 token。
- 日志、运行时数据库、本地备份。
- 私有维护记录。

发布前仍建议按自己的环境再扫描一遍域名、IP 和凭据关键词。

## 开发验证

后端测试：

```bash
cd go-backend
go test ./internal/http/handler
go test ./socket -run "TestRenderEntryDemux|TestNormalizeEntryDemux|TestRenderCoverSite"
```

前端构建：

```bash
cd vite-frontend
npm install --legacy-peer-deps
npm run build
```

## 许可证

本包保留上游 Apache License 2.0 相关文件：

- `LICENSE`
- `LICENSE-APACHE`
- `NOTICE`
