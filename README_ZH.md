<div align="center">

<a href="https://github.com/jonssonyan/h-ui"><img src="./docs/images/head-cover.png" alt="H UI" width="150" /></a>

<h1 align="center">H UI</h1>

[English](README.md) / 简体中文

仅仅是 Hysteria2 的面板

<p>
<a href="https://www.gnu.org/licenses/gpl-3.0.html"><img src="https://img.shields.io/github/license/jonssonyan/h-ui" alt="License: GPL-3.0"></a>
<a href="https://github.com/jonssonyan/h-ui/stargazers"><img src="https://img.shields.io/github/stars/jonssonyan/h-ui" alt="GitHub stars"></a>
<a href="https://github.com/jonssonyan/h-ui/forks"><img src="https://img.shields.io/github/forks/jonssonyan/h-ui" alt="GitHub forks"></a>
<a href="https://github.com/jonssonyan/h-ui/releases"><img src="https://img.shields.io/github/v/release/jonssonyan/h-ui" alt="GitHub release"></a>
<a href="https://hub.docker.com/r/jonssonyan/h-ui"><img src="https://img.shields.io/docker/pulls/jonssonyan/h-ui" alt="Docker pulls"></a>
</p>

![cover](./docs/images/cover.png)

</div>

## 主要功能

- 轻量级、资源占用低、易于部署
- 监控系统状态和 Hysteria2 状态
- 限制用户流量、用户在线状态、强制用户下线、在线用户数、重设用户流量
- 限制用户同时在线设备数、在线设备数量
- 普通用户仅使用独立连接密码，不能登录管理员面板
- 用户订阅链接、节点URL、导入和导出用户
- 支持账户、流量、状态、IPv4 和 IPv6 变量的订阅节点名称与服务器地址模板
- 管理 Hysteria2 配置和 Hysteria2 版本、端口跳跃
- 更改 Web 端口、修改 Hysteria2 流量倍数
- 查看、导入和导出系统日志和 Hysteria2 日志
- 多国语言支持: English, 简体中文
- 页面适配、支持夜间模式、自定义页面主题
- 更多功能等待你发现

## 建议系统

系统: CentOS 8+/Ubuntu 20+/Debian 11+

CPU: x86_64/amd64（D10 安装器暂不支持 ARM64）

内存: ≥ 256MB

## 部署

### 快速安装 (推荐)

在新的 Linux AMD64/systemd 服务器安装最新稳定定制版：

```bash
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/h-ui/main/install.sh | sudo bash
```

安装指定版本或最新预发布版本：

```bash
curl -fsSL https://github.com/GlareGlimmering/h-ui/releases/download/v0.0.25-custom.8-rc.10/install.sh | sudo bash -s -- --version v0.0.25-custom.8-rc.10
curl -fsSL https://github.com/GlareGlimmering/h-ui/releases/download/v0.0.25-custom.8-rc.10/install.sh | sudo bash -s -- --prerelease
```

安装器会在安装前校验 `SHA256SUMS` 和 h-ui 的精确版本，使用 h-ui 内统一的版本元数据安装固定的 `v2.13.0-hui.2` 定制 HY2 内核，初始化 Xray 数据目录，创建 `h-ui.service`，并检查面板首页和前端入口资源。Xray 本身仍为可选项，可在「出站节点」页面安装。

首次安装会只显示一次随机管理员用户名和密码，请立即保存。之后可通过以下命令重置：

```bash
cd /usr/local/h-ui && sudo ./h-ui reset
```

### 手动 systemd 参考

Release 下载：https://github.com/GlareGlimmering/h-ui/releases

```bash
mkdir -p /usr/local/h-ui/
curl -fsSL https://github.com/GlareGlimmering/h-ui/releases/latest/download/h-ui-linux-amd64 -o /usr/local/h-ui/h-ui && chmod +x /usr/local/h-ui/h-ui
curl -fsSL https://raw.githubusercontent.com/GlareGlimmering/h-ui/main/h-ui.service -o /etc/systemd/system/h-ui.service
# 自定义 Web 端口，默认 8081
# sed -i "s|^ExecStart=.*|ExecStart=/usr/local/h-ui/h-ui -p 8081|" "/etc/systemd/system/h-ui.service"
systemctl daemon-reload
systemctl enable h-ui
systemctl restart h-ui
```

旧安装器会把程序卸载与永久删除数据混在一起。安全的 `uninstall`/`uninstall --purge` 命令留待后续实现，D10 不新增卸载命令。手工删除前必须备份 `/usr/local/h-ui/data` 和 `/var/lib/h-ui`。

### 容器部署

1. 安装 Docker

   https://docs.docker.com/engine/install/

   ```bash
   bash <(curl -fsSL https://get.docker.com)
   ```

2. 启动容器

   ```bash
   docker pull jonssonyan/h-ui

   docker run -d --cap-add=NET_ADMIN \
     --name h-ui --restart always \
     --network=host \
     -v /h-ui/bin:/h-ui/bin \
     -v /h-ui/data:/h-ui/data \
     -v /h-ui/export:/h-ui/export \
     -v /h-ui/logs:/h-ui/logs \
     jonssonyan/h-ui
   ```

   自定义 Web 端口，默认 8081

   ```bash
   docker run -d --cap-add=NET_ADMIN \
     --name h-ui --restart always \
     --network=host \
     -v /h-ui/bin:/h-ui/bin \
     -v /h-ui/data:/h-ui/data \
     -v /h-ui/export:/h-ui/export \
     -v /h-ui/logs:/h-ui/logs \
     jonssonyan/h-ui \
     ./h-ui -p 8081
   ```

   设置时区，默认 Asia/Shanghai

   ```bash
   docker run -d --cap-add=NET_ADMIN \
     --name h-ui --restart always \
     --network=host \
     -e TZ=Asia/Shanghai \
     -v /h-ui/bin:/h-ui/bin \
     -v /h-ui/data:/h-ui/data \
     -v /h-ui/export:/h-ui/export \
     -v /h-ui/logs:/h-ui/logs \
     jonssonyan/h-ui
   ```

卸载

```bash
docker rm -f h-ui
docker rmi jonssonyan/h-ui
rm -rf /h-ui
```

## 默认安装信息

- 面板端口: 8081
- SSH 本地转发端口: 8082

自动安装([install.sh](install.sh)):

- 登录用户名/密码: 随机6位字符
- 连接密码: {登录用户名}.{登录密码}

手动安装:

- 登录用户名/密码: sysadmin/sysadmin
- 连接密码: sysadmin.sysadmin

注意: 只有系统管理员可以登录

## 系统升级

升级到最新稳定版、最新预发布版或指定版本：

```bash
sudo /usr/local/h-ui/h-ui upgrade
sudo /usr/local/h-ui/h-ui upgrade --prerelease
sudo /usr/local/h-ui/h-ui upgrade v0.0.25-custom.8-rc.10
```

可增加 `--yes` 跳过确认。升级器会在停止现有服务前完成下载、校验和、执行权限及候选版本精确校验。升级前会在 `/var/lib/h-ui/backups` 中保存 SQLite 一致性备份、旧二进制、HY2 配置及相关 Xray 配置，并只保留最近五份。新版本 systemd 启动或面板/前端资源健康检查失败时，会自动恢复并验证旧版本。

如需手工恢复，选择最新的 `upgrade-*` 快照，并按其中 `manifest.json` 记录的原路径与备份文件映射操作。停止 `h-ui.service`，把 `file-0-h-ui` 恢复到 `/usr/local/h-ui/h-ui`、把 `h_ui.db` 恢复到 `/usr/local/h-ui/data/h_ui.db`，再恢复其他映射文件并启动服务。正常情况下自动回滚已足够；手工操作前请另存一份快照。

## 常见问题

[简体中文 > 常见问题](./docs/FAQ_ZH.md)

## 性能优化

- 定时重启服务器

    ```bash
    0 4 * * * /sbin/reboot
    ```

- 安装网络加速
    - [TCP Brutal](https://github.com/apernet/tcp-brutal) (推荐)
    - [teddysun/across#bbrsh](https://github.com/teddysun/across#bbrsh)
    - [Chikage0o0/Linux-NetSpeed](https://github.com/ylx2016/Linux-NetSpeed)
    - [ylx2016/Linux-NetSpeed](https://github.com/ylx2016/Linux-NetSpeed)

## 客户端

https://v2.hysteria.network/zh/docs/getting-started/3rd-party-apps/

## 开发

Go >= 1.20, Node.js >= 18.12.0

- frontend

   ```bash
   cd frontend
   pnpm install
   npm run dev
   ```

- backend

   ```bash
   go run main.go
   ```

## 构建

- frontend

   ```bash
   npm run build:prod
   ```

- backend

  Windows: [build.bat](build.bat)

  Mac/Linux: [build.sh](build.sh)

## 其他

你可以在 YouTube 上订阅我的频道: https://www.youtube.com/@jonssonyan

## 贡献者

在这里感谢所有为此项目做出贡献的人

<a href="https://github.com/jonssonyan/h-ui/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=jonssonyan/h-ui" />
</a>

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=jonssonyan/h-ui&type=Date)](https://star-history.dera.page/#jonssonyan/h-ui&Date)

## 开源协议

[GPL-3.0](LICENSE)
