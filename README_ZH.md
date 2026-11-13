# Better HY2

A lightweight, modern management panel for Hysteria2.

轻量、现代的 Hysteria2 管理面板。

[English](README.md) · [下载](https://github.com/GlareGlimmering/better-hy2/releases) · [安装与运维](docs/OPERATIONS_ZH.md)

## 功能

- Hysteria2 服务、配置及证书管理。
- DIRECT、原生 SOCKS5 和通过 Xray 适配的 VLESS / HTTP 出站节点。
- 按账户绑定出站，保留严格 Fail-Closed 校验。
- 流量统计、配额、到期和设备限制。
- 订阅模板及服务器公网 IPv4 自动检测。
- 响应式仪表盘、账户抽屉及中英俄三语界面。
- 系统日志、HY2 内核及访问日志，支持轮转与管理。
- SHA256 / 版本校验、数据库备份、启动或健康检查失败自动回滚。

## 环境要求

Linux AMD64、systemd、root 安装权限。支持目标为 Ubuntu / Debian 风格的 systemd 环境；本版本不认证其他发行版或容器安装。需要 Bash、curl、coreutils，以及访问 GitHub 和固定内核 Release 的网络能力。

## 一行安装

```bash
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/better-hy2/main/install.sh | sudo bash
```

默认只解析最新正式版，下载精确版本附件，检查 SHA256 与二进制版本，安装固定定制 HY2 并初始化随机管理员。凭据仅输出一次，请妥善保存。全新或仅 DIRECT 的安装不会自动下载 Xray，Xray 仍为可选功能。

面板仅监听 **http://127.0.0.1:8000**。通过 HTTPS 反向代理或 Cloudflare Tunnel 转发到此地址；安装器不会开放防火墙或配置反向代理。HY2 UDP 监听在面板内单独配置。

## 一行升级

```bash
sudo hui update
```

默认仅升级稳定版并禁止自动降级。保留旧入口：

```bash
sudo /usr/local/h-ui/h-ui upgrade
```

## 从 RC.23 迁移

旧 RC.23 无法解析新的稳定版本号，**不要直接执行旧程序的 `upgrade v1.0.0`**。使用新安装器：

```bash
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/better-hy2/main/install.sh | sudo bash -s -- --version v1.0.0
```

安装器校验新版候选程序后，调用新版升级器迁移。原有账户、绑定、证书、配置和目录保持兼容。升级前备份；面板与 HY2 重启会短暂中断连接。面板升级器不重启独立 Xray。详见[运维与恢复](docs/OPERATIONS_ZH.md)。

## 文档

[安装、首次设置、HY2 配置、Xray 出站、账户绑定、备份恢复及故障排查](docs/OPERATIONS_ZH.md)。

[预发布验收记录与测试进度](docs/ACCEPTANCE_ZH.md)。

为保持兼容，内部 Go module、二进制、数据库和服务继续使用 `h-ui` 名称。本项目不修改 HY2 / Xray 代理协议。匹配内核固定为 **v2.13.0-hui.4**，独立发布于 [GlareGlimmering/hysteria](https://github.com/GlareGlimmering/hysteria)。启用用户路由时不得换成不兼容官方内核。

## 验证与限制

发布前，运营者已确认 RC.23 基线及既有代理链路通过 VPS 人工验收。自动化测试覆盖配置、账户路由约束、安装、升级回滚、前端及嵌入资源；不代表完整安全审计或任何异常下绝对零泄漏证明。UDP 端到端能力取决于节点和上游，TCP 成功不能等同 UDP 成功。新版品牌安装器及升级仍建议先灰度验收。本项目不会自动部署 VPS。

## 构建

使用 `go.mod` 指定的 Go、Node 22 及 `frontend/package.json` 指定的 pnpm：

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm run test:unit
pnpm run build:prod
cd ..
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X h-ui/model/constant.Version=v1.0.0" -o h-ui-linux-amd64 .
```

必须先构建前端：Go 通过 `all:dist` 嵌入全部资源，包括下划线开头的 Chunk，发布流水线检查资源引用完整性。

## 致谢与许可证

基于 [jonssonyan/H UI](https://github.com/jonssonyan/h-ui)。感谢 [Hysteria](https://github.com/apernet/hysteria)、[Xray-core](https://github.com/XTLS/Xray-core) 及上游贡献者。保留原版权和许可要求，参见[第三方声明](THIRD_PARTY_NOTICES.md)。

本项目遵循 [GPL-3.0](LICENSE)，对应版本源码随 Tag 提供；单独下载的代理内核遵循各自许可证。
