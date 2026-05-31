# Agent 资源目录

这个目录会被挂载到前端容器的 `/usr/share/nginx/html/agent`，用于提供内建节点安装包。

`install-public-safe.sh` 默认会从 `go-gost/` 源码构建这些文件：

- `install.sh`
- `gost-amd64`
- `gost-amd64.sha256`
- `gost-arm64`
- `gost-arm64.sha256`

如果你已经在本地生成了 release 资产，也可以手动同步：

```bash
./deploy/sync-agent-assets.sh ./artifacts ./agent
```

如果面板在反向代理或多域名后面，请在面板的网站配置里设置
`agent_download_base_url`，让节点安装脚本使用明确的下载基址。
