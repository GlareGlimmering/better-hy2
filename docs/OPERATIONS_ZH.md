# Better HY2 安装与运维

## 安装与首次设置

使用 [README](../README_ZH.md) 中的一行安装命令。安装器从官方仓库解析精确稳定 Tag，校验 SHA256 和实际版本，安装定制 HY2，在启动服务前初始化 schema 和随机管理员，再创建 systemd 并检查首页及入口 JS。无有效程序/服务的半安装环境会安全续装；有效旧安装交给新版候选程序升级。Release 不包含数据库或凭据。

保留 `h-ui.service`、`/usr/local/h-ui/h-ui`、`HUI_DATA=/usr/local/h-ui/`、`/usr/local/h-ui/data/h_ui.db` 和 `/var/lib/h-ui`。`/usr/local/bin/hui` 是指向旧二进制路径的链接，绝不覆盖其他软件的同名命令。

面板仅监听 `http://127.0.0.1:8000`，通过 Caddy/Nginx HTTPS 或 Cloudflare Tunnel 转发到本机地址。首次访问也可通过 SSH 本地转发。不要将裸 HTTP 暴露公网；安装器不会设置防火墙或反向代理。品牌更名不要求更换证书、数据库和配置目录。

从任意工作目录重置管理员：

```bash
sudo /usr/local/h-ui/h-ui reset
```

自定义管理员凭据可以使用：

```bash
sudo hui reset --interactive
sudo hui reset username
sudo hui reset password
```

第一条交互设置用户名和密码，后两条只修改对应字段。用户名也可直接作为 `hui reset username <用户名>` 的参数。密码输入不会回显，脚本可用 `hui reset --username <用户名> --password-stdin` 或 `hui reset password --stdin` 从标准输入读取一行。直接运行 `hui reset` 仍保留现有管理员名称并生成随机密码。

普通代理账户会保留。显式 `HUI_DATA` 可选择其他数据根目录，生产环境不要误设。


## RC.4 及后续版本的面板管理员

升级会保留有效管理员的用户名。若旧密码曾同时用于代理，请先通过 SSH 设置新面板密码。如果别人知道旧密码，请先在本机执行 `sudo hui reset`。重置会保留已有用户名并作废该管理员的会话。通过 SSH 执行 `sudo hui reset password` 可设置自定义密码。

仍在用旧管理员凭据代理的客户端，请在升级前改用独立代理账户。普通代理账户和订阅继续有效。账户导入和导出功能已移除，历史文件会保留，可能含有连接凭据，请妥善处理。

回退到 RC.3 时，请一并恢复升级前的 SQLite 快照、程序和配置。RC.3 无法从已迁移数据库校验面板管理员。

### 反代、订阅和登录限流

现有 HTTPS 域名、面板前缀和订阅 URL 继续使用。反代只转发到面板端口并保留路径前缀。如有旧的 `POST <前缀>/hui/hysteria2/auth` 专用转发规则，请移除。新版在内核启动前生成独立的本机回调地址。

面板只信任 `HUI_TRUSTED_PROXIES` 显式列出的 IP 或 CIDR，多值用逗号分隔。可信的直接代理才能通过 `X-Forwarded-For` 提供客户端地址。变量未设置时，来自同一代理的请求共用登录额度。登录和换密按 IP 共用独立额度，突发最多 5 次，每秒补充 1 次。单层本机 Nginx 配置见[英文运维示例](OPERATIONS.md#reverse-proxy-subscriptions-and-login-limits)。

订阅响应带 `Cache-Control: no-store`。在 Nginx 面板站点可设置 `proxy_cache off` 和 `access_log off`。如果 Cloudflare 或其他 CDN 的规则可能缓存订阅，按面板域名或包含前缀的订阅路径设置绕过缓存，按需清除旧缓存。访问日志、错误日志和追踪应省略订阅的原始 URL，可记录固定路由名、方法和状态码。

## ACME Worker 联调版

`v2.0.2-rc.1` 配套 `v2.13.0-hui.6-dev`。在全新隔离 Linux AMD64 systemd 环境中运行该 RC 自带的 `install.sh --version v2.0.2-rc.1`，安装器会校验并装入配套内核。已安装旧内核的环境会被 `hui update` 拦截，需在维护模式成对替换和恢复。

在 HY 管理选择 ACME 的 DNS 模式和 `worker`，在同一个输入框填写 HTTPS 根地址、空格和节点凭据。一次复制会复制完整凭据，并要求先确认。用独立测试域名、独立 `acme.dir` 和 `letsencrypt-staging` 验证首次签发与实际重新验证 DNS-01 的续期。测试域名对应的 TXT 必须在节点白名单内。生产切换沿用原证书域名与 ACME 目录，保留原 CA。旧直连 Token 可在联调和灰度通过后撤销。

## HY2 配置

在 HY2 管理设置监听、TLS/ACME 和网络选项。使用当前 Release 的 `HY2-PAIR.json` 指定的定制内核。HTTP Auth 与 `userOutbounds` 由面板维护，不要手改覆盖。保留定制内核的 ACME IPv4 DNS 传播逻辑，不全局禁用 IPv6，不关闭证书验证。

订阅节点默认使用检测到的公网 IPv4；只有 NAT、接口无公网地址时需显式配置 `HUI_NODE_ADDRESS`。HTTP 订阅 URL、节点地址与 TLS SNI 是独立概念。

## 生成 HY2 自建证书

使用自签 CA 签发 HY2 服务证书。命令见[自建证书示例](OPERATIONS.md#generate-a-private-hy2-certificate)。把 `hy2-server.crt` 和 `hy2-server.key` 用于 HY 管理的 TLS 表单，只向客户端分发 `hy2-ca.crt`。客户端 SNI 使用证书 SAN 中的名称，连接地址可以使用 VPS 公网 IP。服务证书应在到期前续签，CA 私钥离线保存。

## 出站和账户绑定

DIRECT 内置；原生 SOCKS5 无需 Xray。VLESS/HTTP 节点在「出站节点」配置，先用「安装/更新内核」安装固定 Xray 版本。节点保存时会验证并应用配置。每个适配节点使用独立 loopback SOCKS5 端口，不允许监听公网。HTTP CONNECT 不代表 UDP 支持。

在账户抽屉选择启用节点并保存。严格路由下，每个有效用户都需明确绑定（DIRECT 也是显式绑定）。失效绑定不会自动改成 DIRECT；被引用节点不能删除/禁用。TCP、UDP 单独测试，最近测试不代表实时在线。无法识别归属的历史 HY2 规则保留在高级设置，继续保留 RC.23 幽灵节点选择器保护。

## 升级与恢复

目标版本的安装器会调用已验证的新版更新器，可供仍在使用旧更新器的实例进入升级。新版更新器在停机前验证发布制品。哈希相同仅更新面板，哈希不同时验证已安装的源配套并进行双制品维护升级。这条路径当前适用于正在运行的标准 v2 安装。已运行 v2 时，`sudo hui update` 选择最新的 v2 正式版，也保留旧 `upgrade` 命令。使用原配套内核的 v1.0.0 可继续运行 v1，或在另一台主机全新安装 v2。

升级快照位于 `/var/lib/h-ui/backups/upgrade-*`，包含 SQLite 一致性备份、旧程序、systemd unit、HY2 YAML、相关 Xray 配置/CA 及文件所有权信息，保留最新五份。启动或 HTTP/入口资源检查失败会自动恢复快照与原运行状态；回滚失败会明确报错。配套内核匹配时更新器只替换面板。

升级前仍建议额外私密备份 `/usr/local/h-ui`、`/var/lib/h-ui` 和 unit，保留权限；原始文件拷贝应停服务以确保一致性，或使用 SQLite 专用备份。备份含敏感数据，不要公开上传。手工恢复时停止 `h-ui.service`，另存失败状态，依照 `manifest.json` 恢复每个文件到原路径并恢复权限、所有权，daemon-reload 后恢复原服务状态。数据库恢复会丢弃快照之后的修改。链接仍指向恢复后的程序，旧版本无 `update` 时使用 `hui upgrade`。

## 从 v1.0.0 显式迁移到 v2

适用于官方 v1.0.0 面板与官方 v2.13.0-hui.4 内核的 Linux AMD64 标准安装。服务须正在运行，使用标准路径和 8000 端口，且没有 systemd 覆盖配置。请提前额外保留私有备份。

```bash
installer=$(mktemp) &&
curl -fsSL https://github.com/GlareGlimmering/better-hy2/releases/download/v2.0.1/install.sh -o "$installer" &&
sudo bash "$installer" --version v2.0.1 --migrate-v1
```

旧服务运行时下载并校验配套制品，准备恢复快照。停机从停止 h-ui.service 及 HY2 子进程开始，随后刷新 SQLite 备份、成对替换程序、迁移数据库和受管配置，并检查面板与 HY2。启动或健康检查失败时恢复旧组合、SQLite 和配置，重新启动原 unit。已落库计数会保留，停止时仍留在旧内核内存中的未结算流量可能丢失。若遇断电或强制终止，先按输出的快照路径进行手工恢复，再重试。

原管理员可用原用户名登录。如果旧密码与代理凭据共用，先通过 SSH 执行 `sudo hui reset password`。需要在本机找回登录时执行 `sudo hui reset`。仍用旧管理员凭据代理的客户端应在迁移前换用普通代理账户。迁移后刷新原订阅，并新建一次真实 HY2 连接验证。

## 卸载

执行 `sudo hui uninstall`，核对清单后输入 `UNINSTALL`。直接回车取消。默认停用并移除标准面板服务、受管 HY2、匹配的 `hui` 链接、标准安装目录、升级备份及 HY2 访问日志（含轮转文件）。旧参数 `--purge` 执行相同操作，确认词为 `PURGE`。

通过归属检查的配套 Xray 服务、运行文件和专用系统账号也会移除。独立或自定义服务及其关联文件会保留。卸载范围不包括外部证书、共享 systemd journal、外部备份及 Cloudflare 服务。文件删除不覆盖磁盘块。需要保留数据时，确认前请在清理范围外单独备份；若有失败，按错误提示处理后重试。

删除代理账号时，SQLite 启用安全删除。已经存在的备份、历史日志及旧空闲页或日志文件的残留，仍按各自的留存规则处理。

## 故障排查

```bash
systemctl status h-ui --no-pager
journalctl -u h-ui -n 100 --no-pager
curl -fsS http://127.0.0.1:8000/
sudo /usr/local/h-ui/h-ui version
```

白屏时核对 HTML 中入口 JS。面板健康不代表 HY2/出站可用。查看 `/usr/local/h-ui/logs/h-ui.log`、`hysteria2.log`、`/var/lib/hui/hy2_access.log`；访问日志需要真实代理请求。只分享脱敏认证诊断，不分享密码和 Token。ACME 应查看真实 fatal 原因，不要通过关闭 TLS 验证规避。节点不可用、用户未绑定时应保持拒绝转发。

## 发布校验

从同一精确版本下载 `h-ui-linux-amd64`、`hysteria-linux-amd64`、`HY2-PAIR.json`、`install.sh` 和 `SHA256SUMS`。将五个文件放在同一目录，执行 `sha256sum --check SHA256SUMS`。`./h-ui-linux-amd64 version` 的输出应与发布标签一致。CI 使用隔离的 systemd 环境，真实部署请另外验收自己的网络、反代与 HY2 客户端。
