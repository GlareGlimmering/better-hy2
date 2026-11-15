# Better HY2

Better HY2 是用于管理 Hysteria2 服务、代理账户和出站节点的面板。面板和订阅可以经 Cloudflare Tunnel 提供 HTTPS，HY2 客户端直接通过 UDP 连接 VPS。

[English](README.md) · [下载 v2.0.1](https://github.com/GlareGlimmering/better-hy2/releases/tag/v2.0.1) · [运维文档](docs/OPERATIONS_ZH.md)

## 做了什么

- **代理账户与订阅**。支持到期时间、同时在线连接数、GB 流量配额和每账户共享的上下行限速（Mbps）。仪表盘显示流量，订阅支持自定义节点名称和连接地址，默认读取服务器公网 IPv4。
- **统一出站节点**。账户可绑定 DIRECT、原生 SOCKS5 或由 Xray 转发的 VLESS、HTTP 节点。严格路由下，缺少绑定或指定节点故障时拒绝转发。节点支持分开测试 TCP 和 UDP。
- **面板安全**。管理员与代理账户独立，使用带盐 Argon2id、登录限流、可信反代和改密后会话撤销。HY2 认证回调使用独立的本机入口，原有订阅地址可继续使用，订阅响应要求禁用缓存。
- **部署与排查**。从面板管理 HY2 与 TLS，查阅内核日志、访问日志和流量数据。CLI 支持安装、校验升级、快照回退和卸载。Release 提供经过配套验证的内核及兼容清单。

## 安装 v2.0.1

需要 **Linux AMD64**、**systemd**、Bash、curl、coreutils 和 root 权限。安装器校验面板版本与制品哈希，并安装配套内核。全新安装会输出一次随机管理员凭据。使用 DIRECT 无需安装 Xray。

```bash
installer=$(mktemp) &&
curl -fsSL https://github.com/GlareGlimmering/better-hy2/releases/download/v2.0.1/install.sh -o "$installer" &&
sudo bash "$installer" --version v2.0.1
```

标准安装的面板监听 `http://127.0.0.1:8000`。反代和防火墙由操作者设置。

## 推荐部署：Cloudflare Tunnel + 自建证书

| 用途 | 公网入口 | VPS 上的服务 |
| --- | --- | --- |
| 面板和订阅 | `https://panel.example.com` | Cloudflare Tunnel → `http://127.0.0.1:8000` |
| HY2 代理流量 | VPS 公网 IP 或仅 DNS 解析的域名，UDP `443` | Hysteria2 监听端口 |
| HY2 认证回调 | 本机使用 | Better HY2 管理的独立回环入口 |

1. 在同一台 VPS 运行 `cloudflared`，将 `panel.example.com` 发布为 HTTP 应用，上游填 `http://127.0.0.1:8000`。公网 HTTPS 由 Cloudflare 提供。若缓存规则会匹配订阅，设置绕过缓存，并避免在代理日志中记录完整订阅 URL。[Cloudflare 应用发布文档](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/routing-to-tunnel/) · [反代配置](docs/OPERATIONS_ZH.md#反代订阅和登录限流)
2. 在 **HY 管理 → TLS 与证书** 填入 HY2 服务证书和私钥，监听地址可用 `:443`。自建证书可由自签 CA 签发，证书 SAN 与 HY2 的 SNI 一致，例如 `hy2.example.com`。在 VPS 和云防火墙放行 **UDP 443**。Tunnel 承载面板与订阅，客户端的 HY2 流量走独立的 UDP 入口。[证书生成命令](docs/OPERATIONS.md#generate-a-private-hy2-certificate)
3. 创建一个普通代理账户，先绑定 **DIRECT**，再启动 HY2。开启严格路由前，为所有有效代理账号设置明确的出站绑定。可复制订阅 URL，或使用下面的原生 HY2 客户端配置。使用私有 CA 的客户端需要配置 CA 和 SNI。客户端无法导入私有 CA 或校验指纹时，可以使用公开受信的 HY2 证书。[Hysteria 客户端 TLS 参数](https://v2.hysteria.network/docs/advanced/Full-Client-Config/#tls)

```yaml
server: 198.51.100.10:443
auth: "在此填写完整代理连接凭据"
tls:
  sni: hy2.example.com
  ca: ./hy2-ca.crt
  insecure: false
socks5:
  listen: 127.0.0.1:1080
```

把示例 IP 换成 VPS 公网 IP。客户端只需要 CA 公共证书。管理员账号用于登录面板，代理账号使用独立的连接凭据。

## 常用指令

| 操作 | 指令 |
| --- | --- |
| 查看面板版本 | `hui version` |
| 查看内核版本和能力 | `/usr/local/h-ui/bin/hysteria-linux-amd64 version --json` |
| 本机恢复管理员访问 | `sudo hui reset` |
| 更新现有 v2 到同系列正式版 | `sudo hui update` |
| 指定 v2 版本 | `sudo hui update v2.0.1` |
| 查看面板服务状态 | `systemctl status h-ui.service --no-pager` |
| 跟踪服务日志 | `sudo journalctl -u h-ui.service -f` |
| 跟踪 HY2 请求日志 | `sudo tail -F /var/lib/hui/hy2_access.log` |
| 重启面板及其受管子进程 | `sudo systemctl restart h-ui.service` |
| 卸载面板并保留数据 | `sudo hui uninstall` |
| 查看完整清除计划 | `sudo hui uninstall --purge` |

单独启停或重启 HY2，请在 **HY 管理** 操作。CLI 卸载会先列出路径并要求确认。

## 升级到 v2.0.1

- **已运行 v2，包括 v2.0.1-rc.1**：执行 `sudo hui update v2.0.1`。更新器会核对已安装内核与目标配套清单，再重启服务。
- **官方 v1.0.0 与 v2.13.0-hui.4**：按[显式迁移流程](docs/OPERATIONS_ZH.md#从-v100-显式迁移到-v2)操作。GitHub 的旧 Latest 入口继续供 v1 更新器使用。
- **回退到较低主版本**：新版更新器和安装器会在下载目标程序和停止服务前拒绝。升级失败时仍可用原程序及对应数据库快照恢复。旧版安装脚本和手工替换文件不受新版命令约束。

v2.0.1 配套的是 Release 中精确构建的 `v2.13.0-hui.5-dev`，支持 Traffic Stats 协议 2 和账号限速能力 1。内核版本沿用已测试 RC 的标识。[备份和恢复](docs/OPERATIONS_ZH.md)

## 构建与致谢

使用 `go.mod` 所需的 Go、Node 22 和 `frontend/package.json` 固定的 pnpm。先构建前端，再编译 Go 程序，前端资源会嵌入二进制。

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run test:unit
pnpm run build:prod
cd ..
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-X h-ui/model/constant.Version=v2.0.1" -o h-ui-linux-amd64 .
```

基于 [H UI](https://github.com/jonssonyan/h-ui)，感谢 [Hysteria](https://github.com/apernet/hysteria) 和 [Xray-core](https://github.com/XTLS/Xray-core) 贡献者。[GPL-3.0](LICENSE) · [第三方许可说明](THIRD_PARTY_NOTICES.md) · [验收记录](docs/ACCEPTANCE_ZH.md)
