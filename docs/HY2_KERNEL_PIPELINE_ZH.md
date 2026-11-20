# Better HY2 内核候选流水线

面板的标准安装、内核下载和配套 CI 共享 [已批准的内核记录](../model/constant/hy2-pair.json)。候选流水线读取该记录，获取指定的官方标签及已批准的定制提交，在独立目录试合并。不会改变该记录或服务器。

## 生成候选

需要 Linux AMD64、Git（支持 merge-tree --write-tree）、Python 3.11 以上、uv 及新内核要求的 Go。Go race detector 还需要 C 编译器。相关测试依赖 iproute2、iptables 和 openssl。Windows 下在 WSL 的原生 Linux 文件系统中运行，以支持内核测试使用的 Unix socket。

从面板仓库执行以下命令。工作目录必须不存在。

```bash
work="$(mktemp -d)/candidate"
python3 scripts/hy2-candidate.py prepare "$work" --tag v2.14.0 &&
python3 scripts/hy2-candidate.py verify "$work"
```

成功时，report/ 记录官方标签、两边的提交、合并结果和相对官方源码的补丁。verified/ 包含 Linux AMD64 二进制、SHA256SUMS 和候选清单。制品只有在版本、提交、能力字段和内核全套测试、Go vet 和核心并发回归通过后才进入 verified/。候选清单记录编译器版本，便于复现二进制。GitHub Actions 构建使用候选源码声明的 Go 版本。

如果有冲突，prepare 返回 2，并在 report/merge.txt 中列出冲突文件，不进入构建阶段。在内核 fork 的独立分支合并同一个官方标签，解决冲突，测试并推送该分支。用新的工作目录，加上 --candidate 和已审核的完整提交 SHA 重跑。脚本会检查该提交包含官方标签及此前批准的 Better HY2 内核。推送前也可以加上 --fork-dir <本地内核工作树> 进行同样的验证。需要改变候选版本号时传 --version v2.14.0-hui.2-dev。省略 --tag 可重跑当前批准基线。

工作流进入默认分支后，可在 GitHub Actions 的 Better HY2 kernel candidate 手动输入标签、已审查的候选提交和候选版本。合入前，本地命令可完整运行，开发分支上的工作流改动触发一次批准基线验证。工作流仅上传内部构建产物，不创建 Release。

## 接入面板前的验证

候选通过内核测试后，再验证与面板的兼容性。特别检查 HTTP 认证、TCP/UDP 用户级出站与 Fail-Closed、计量批次确认与重试、连接限制、踢线、账号限速，以及 Worker DNS-01 和传统 Cloudflare DNS-01。使用面板已有的配套制品和 systemd 回归，另外验证实际域名请求不会先从本机解析器发出。

只在候选与面板确认兼容后才更新内核记录的 upstreamTag、upstreamCommit、commit、version 和 sha256。无冲突时，准备阶段会在本地创建可复现的合并提交。批准前应将该提交推送为内核 fork 的独立分支，否则配套发布 CI 无法检出它。版本和 SHA256 应来自目标提交在 GitHub Actions 生成的候选清单。保持现有配套校验。当前 hui update 会拒绝安装与目标配套内核哈希不同的版本。不同内核的部署和回滚需要独立验收。
