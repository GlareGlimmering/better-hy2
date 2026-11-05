# Better HY2 安装与运维

## 安装与首次设置

使用 [README](../README_ZH.md) 中的一行安装命令。安装器从官方仓库解析精确稳定 Tag，校验 SHA256 和实际版本，安装定制 HY2，在启动服务前初始化 schema 和随机管理员，再创建 systemd 并检查首页及入口 JS。无有效程序/服务的半安装环境会安全续装；有效旧安装交给新版候选程序升级。Release 不包含数据库或凭据。

保留 `h-ui.service`、`/usr/local/h-ui/h-ui`、`HUI_DATA=/usr/local/h-ui/`、`/usr/local/h-ui/data/h_ui.db` 和 `/var/lib/h-ui`。`/usr/local/bin/hui` 是指向旧二进制路径的链接，绝不覆盖其他软件的同名命令。手工卸载时只在核实链接目标后删除该链接，不提供自动清空数据功能。

面板仅监听 `http://127.0.0.1:8000`，通过 Caddy/Nginx HTTPS 或 Cloudflare Tunnel 转发到本机地址。首次访问也可通过 SSH 本地转发。不要将裸 HTTP 暴露公网；安装器不会设置防火墙或反向代理。品牌更名不要求更换证书、数据库和配置目录。

从任意工作目录重置管理员：

```bash
sudo /usr/local/h-ui/h-ui reset
```

不会删除普通代理账户。显式 `HUI_DATA` 可选择其他数据根目录，生产环境不要误设。

## HY2 配置

在 HY2 管理设置监听、TLS/ACME 和网络选项。使用固定定制内核 `v2.13.0-hui.4`，SHA256：`f1577bff290f5591a651b88596103045390581a6c0a6e999611cab8baa653d9e`。HTTP Auth 与 `userOutbounds` 由面板维护，不要手改覆盖。保留定制内核的 ACME IPv4 DNS 传播逻辑，不全局禁用 IPv6，不关闭证书验证。

订阅节点默认使用检测到的公网 IPv4；只有 NAT、接口无公网地址时需显式配置 `HUI_NODE_ADDRESS`。HTTP 订阅 URL、节点地址与 TLS SNI 是独立概念。

## 出站和账户绑定

DIRECT 内置；原生 SOCKS5 无需 Xray。VLESS/HTTP 节点在「出站节点」配置，需要时安装固定 Xray 内核，填写认证和 CA 后验证、应用。每个适配节点使用独立 loopback SOCKS5 端口，不允许监听公网。HTTP CONNECT 不代表 UDP 支持。

在账户抽屉选择启用节点并保存。严格路由下，每个有效用户都需明确绑定（DIRECT 也是显式绑定）。失效绑定不会自动改成 DIRECT；被引用节点不能删除/禁用。TCP、UDP 单独测试，最近测试不代表实时在线。无法识别归属的历史 HY2 规则保留在高级设置，继续保留 RC.23 幽灵节点选择器保护。

## 升级与恢复

使用 `sudo hui update`，默认只选稳定版，也保留旧 `upgrade` 命令。RC.23 不接受新版本号，必须通过新安装器 `--version v1.0.0` 迁移。候选下载、校验和版本验证完成后才停止服务。标准安装保持 loopback 8000，不修改防火墙。

升级快照位于 `/var/lib/h-ui/backups/upgrade-*`，包含 SQLite 一致性备份、旧程序、systemd unit、HY2 YAML、相关 Xray 配置/CA 及文件所有权信息，保留最新五份。启动或 HTTP/入口资源检查失败会自动恢复快照与原运行状态；回滚失败会明确报错。v1.0.0 不改变匹配的 HY2/Xray 内核版本，保留配置、账户绑定和证书路径。

升级前仍建议额外私密备份 `/usr/local/h-ui`、`/var/lib/h-ui` 和 unit，保留权限；原始文件拷贝应停服务以确保一致性，或使用 SQLite 专用备份。备份含敏感数据，不要公开上传。手工恢复时停止 `h-ui.service`，另存失败状态，依照 `manifest.json` 恢复每个文件到原路径并恢复权限、所有权，daemon-reload 后恢复原服务状态。数据库恢复会丢弃快照之后的修改。链接仍指向恢复后的程序，旧版本无 `update` 时使用 `hui upgrade`。

## 故障排查

```bash
systemctl status h-ui --no-pager
journalctl -u h-ui -n 100 --no-pager
curl -fsS http://127.0.0.1:8000/
sudo /usr/local/h-ui/h-ui version
```

白屏时核对 HTML 中入口 JS。面板健康不代表 HY2/出站可用。查看 `/usr/local/h-ui/logs/h-ui.log`、`hysteria2.log`、`/var/lib/hui/hy2_access.log`；访问日志需要真实代理请求。只分享脱敏认证诊断，不分享密码和 Token。ACME 应查看真实 fatal 原因，不要通过关闭 TLS 验证规避。节点不可用、用户未绑定时应保持拒绝转发。

## 发布校验

从同一精确版本 Release 下载 `h-ui-linux-amd64`、`SHA256SUMS`、`install.sh`，执行 `sha256sum --check SHA256SUMS`。清单包含程序和安装器。`./h-ui-linux-amd64 version` 必须与 Tag 一致。CI 不连接私人服务器，本地 fixture 不能替代真实 systemd、反向代理及 UDP 验收。
