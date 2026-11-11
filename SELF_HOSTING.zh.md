<a id="self-hosting-guide"></a>

# 自托管指南

[English](SELF_HOSTING.md) | 简体中文

在自己的基础设施上部署 Multica。

<a id="architecture"></a>

## 架构

| 组件 | 职责 | 技术 |
| --- | --- | --- |
| **后端** | REST API 和 WebSocket 服务 | Go，单一二进制程序 |
| **前端** | Web 应用 | Next.js 16 |
| **数据库** | 主要数据存储 | PostgreSQL 17，使用 `pgcrypto` 和 `pg_trgm` |

需要在本地运行 AI 智能体的用户，还应在自己的机器上安装 **`multica` CLI** 并运行**智能体守护进程**。

<a id="anonymous-deployment-telemetry"></a>

## 匿名部署遥测

自托管 API 服务端每个 UTC 日向 Multica 固定端点 `https://telemetry.multica.ai/v1/telemetry/events` 发送一次由 Multica 直接接收的部署级匿名快照，用于了解使用中的发布版本、部署规模区间、运行总量，以及活跃使用部署的成员数。端点地址编译在服务端中，不能通过配置重定向。

快照仅包含发布版本；过去 24 小时内的工作区、去重后的真人成员、活跃智能体和已出现守护进程的分桶数量；该时间窗口内启动、完成、失败和取消的运行总量；以及过去 1、7、28 个完整 UTC 日中，打开已登录 Web 或桌面应用的成员数、亲自启动智能体运行的成员数，后二者也按区间分桶。亲自启动包括分配、提及、评论、聊天、手动运行或重新运行，不包括自动化和智能体之间的委派。成员按部署统计，不包含成员身份。快照不包含名称、邮箱地址或域名、IP 地址、业务 ID、主机或设备详情、仓库或组织详情、模型或插件数据、提示词或输出、评论或聊天、路径、Token 或费用数据、凭据、错误、堆栈或日志。遥测不用于计费、许可、认证或安全决策。

遥测默认开启。要同时停止采集和网络发送，请在 API 服务端设置 `DO_NOT_TRACK=1` 或 `DO_NOT_TRACK=true`，然后重新创建或重启后端。部署标识仍保存在 PostgreSQL 中，再次启用时不会创建新标识。`ANALYTICS_DISABLED` 控制独立的 PostHog 集成，不会关闭此遥测。

将生产数据库克隆到预发布或测试环境时，应在启动克隆环境的 API 服务端前清空其 `instance_telemetry_state` 表，或设置 `DO_NOT_TRACK=true`。否则两个部署会共用标识，同一天的快照可能被静默去重。

<a id="quick-install-recommended"></a>

## 快速安装 推荐方式

通过两条命令完成服务端、CLI 和连接配置。

<details open>
<summary><b>macOS / Linux</b></summary>

<br/>

```bash
# 1. 安装 CLI 并部署自托管服务端
curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash -s -- --with-server

# 2. 配置 CLI、完成认证并启动守护进程
multica setup self-host
```
</details>
<details>
<summary><b>Windows (PowerShell)</b></summary>

<br/>

```powershell
# 1. 安装 CLI 并部署自托管服务端
$env:MULTICA_MODE="with-server"; irm https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.ps1 | iex

# 2. 配置 CLI、完成认证并启动守护进程
multica setup self-host
```
</details>

安装过程会安装 `multica` CLI，检出最新的自托管部署文件，从 GHCR 拉取官方 Multica 镜像，并配置为通过 localhost 使用。

打开 http://localhost:3000。建议在 `.env` 中配置 `RESEND_API_KEY`，通过邮件接收登录验证码；也可以不配置 Resend，从后端日志复制生成的验证码。详见[步骤 2 登录](#step-2--log-in)。

> **前置条件：** 已安装 Docker 和 Docker Compose。脚本会检查依赖，缺失时提供安装链接。
>
> **只需要 CLI？** 自托管服务端已运行时，可在 macOS／Linux 上通过 Homebrew 安装 CLI：
>
> ```bash
> brew install multica-ai/tap/multica
> ```

---

<a id="step-by-step-setup-alternative"></a>

## 分步部署 备选方式

也可以逐步手动执行：

<a id="step-1--start-the-server"></a>

### 步骤 1 启动服务端

**前置条件：** Docker 和 Docker Compose。

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
make selfhost
```

`make selfhost` 会自动从模板创建 `.env`、生成随机 `JWT_SECRET`，并通过 Docker Compose 启动所有服务。

默认从 GHCR 拉取最新稳定版镜像。需要使用当前检出代码构建后端和前端时，运行 `make selfhost-build`。
如果所选 GHCR 标签尚未发布，`make selfhost` 会提示改用 `make selfhost-build`。
`make selfhost-build` 使用本地 `multica-backend:dev`／`multica-web:dev` 标签，不会覆盖已拉取的 `:latest` 镜像。

服务就绪后：

- **前端：** http://localhost:3000
- **后端 API：** http://localhost:8080

> **说明：** 如需手动执行 Docker Compose 步骤，见下方[手动 Docker Compose 部署](#manual-docker-compose-setup)。

<a id="step-2--log-in"></a>

### 步骤 2 登录

在浏览器中打开 http://localhost:3000。Docker 自托管服务默认使用 `APP_ENV=production`，配置位于 `docker-compose.selfhost.yml`，默认没有固定验证码。可选择以下登录方式：

- **生产环境推荐：** 在 `.env` 中配置 `RESEND_API_KEY`，然后重启后端。系统会向输入的邮箱发送真实验证码。见[高级配置中的邮件设置](SELF_HOSTING_ADVANCED.zh.md#email-required-for-authentication)。
- **未配置邮件：** 服务端生成验证码并输出到后端容器日志，可搜索 `[DEV] Verification code for ...:`，适合单机临时测试。
- **固定验证码的本地或私有测试：** 在 `.env` 中设置 `APP_ENV=development` 和 `MULTICA_DEV_VERIFICATION_CODE=888888`，然后重启后端。`APP_ENV=production` 时忽略固定验证码。

修改 `ALLOW_SIGNUP`、`DISABLE_WORKSPACE_CREATION`、`WORKSPACE_CREATION_ALLOWED_EMAILS` 和 `GOOGLE_CLIENT_ID` 后，也需要重启后端或 Compose 服务。Web 界面在运行时从 `/api/config` 读取 `ALLOW_SIGNUP`、`DISABLE_WORKSPACE_CREATION` 和 `GOOGLE_CLIENT_ID`，无需重新构建前端。`WORKSPACE_CREATION_ALLOWED_EMAILS` 仅由后端读取，不会通过 `/api/config` 暴露。限制工作区创建或仅允许指定用户创建的推荐步骤，见[高级配置中的注册控制](SELF_HOSTING_ADVANCED.zh.md#signup-controls-optional)。

> **警告：** 不要在可公开访问的实例上设置 `MULTICA_DEV_VERIFICATION_CODE`，否则知道邮箱地址的人都能使用固定验证码登录。

<a id="step-3--install-cli--start-daemon"></a>

### 步骤 3 安装 CLI 并启动守护进程

守护进程运行在本机，不在 Docker 内。它检测已安装的 AI 智能体 CLI，将其注册到服务端，并在智能体被分配工作时执行运行。

每位需要在本地运行 AI 智能体的团队成员，都需要完成以下步骤。

<a id="a-install-the-cli-and-an-ai-agent"></a>

### a 安装 CLI 和 AI 智能体

```bash
brew install multica-ai/tap/multica
```

还需至少安装以下一种 AI 智能体 CLI：

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)，PATH 中包含 `claude`。
- [Antigravity CLI](https://antigravity.google/docs/cli-install)，PATH 中包含 `agy`。
- [CodeBuddy Code](https://www.codebuddy.ai/docs/cli/quickstart)，PATH 中包含 `codebuddy`。
- [DevEco Code](https://gitcode.com/openharmony-sig/deveco-code)，PATH 中包含 `deveco`。
- [Codex](https://github.com/openai/codex)，PATH 中包含 `codex`。
- [GitHub Copilot CLI](https://docs.github.com/en/copilot)，PATH 中包含 `copilot`。
- [OpenClaw](https://github.com/openclaw/openclaw)，PATH 中包含 `openclaw`。
- [OpenCode](https://github.com/anomalyco/opencode)，PATH 中包含 `opencode`。
- [Huawei Cloud CodeArts](https://support.huaweicloud.com/qs-codeartssnap/codeartsagent_qs_0004.html)，PATH 中包含 `codearts`。
- [Hermes](https://github.com/NousResearch/hermes)，PATH 中包含 `hermes`。
- [Pi](https://pi.dev/)，PATH 中包含 `pi`。
- [Cursor Agent](https://cursor.com/)，PATH 中包含 `cursor-agent`。
- Kimi，PATH 中包含 `kimi`。
- [Reasonix](https://github.com/esengine/DeepSeek-Reasonix)，PATH 中包含 `reasonix`，先运行 `reasonix setup`。
- Dim，PATH 中包含 `dim`。
- Kiro CLI，PATH 中包含 `kiro-cli`。
- Qoder CLI，PATH 中包含 `qodercli`。
- Qoder CN CLI，PATH 中包含 `qoderclicn`。
- Trae CLI，PATH 中包含 `traecli`。
- [Grok Build CLI](https://docs.x.ai/)，PATH 中包含 `grok`。
- Qwen Code，PATH 中包含 `qwen`。
- [QwenPaw](https://github.com/agentscope-ai/QwenPaw)，PATH 中包含 `qwenpaw`，模型在 QwenPaw 自身配置中选择。
- [MiniMax Code CLI](https://www.npmjs.com/package/@minimax-ai/code)，PATH 中包含 0.1.2 或更高版本的 `mcode`。安装受支持的 Node.js 版本（`>=22.19 <23` 或 `>=24 <27`），运行 `npm install --global @minimax-ai/code@latest`，再执行 `mcode login`。
- [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)，PATH 中包含 `dsh`，已安装 Multica 运行时配置，并设置 `DEEPSEEK_API_KEY`。

<a id="b-one-command-setup"></a>

### b 一条命令完成配置

```bash
multica setup self-host
```

命令会自动：

1. 配置 CLI 连接 `localhost`，端口为 8080／3000。
2. 打开浏览器完成认证。
3. 发现你的工作区。
4. 在后台启动守护进程。

使用自定义域名的本地部署可执行：

```bash
multica setup self-host --server-url https://api.example.com --app-url https://app.example.com
```

验证守护进程是否运行：

```bash
multica daemon status
```

> **备选方式：** 如需逐项配置，见下方[手动配置 CLI](#manual-cli-configuration)。

<a id="step-4--verify--start-using"></a>

### 步骤 4 验证并开始使用

1. 在 http://localhost:3000 的 Web 应用中打开工作区。
2. 进入**设置 → 运行时**，确认列表中出现你的机器。
3. 进入**设置 → 智能体**，创建智能体。
4. 创建任务并分配给该智能体，它会自动接手执行。

---

<a id="kubernetes-deployment-alternative"></a>

## Kubernetes 部署 备选方式

已有 Kubernetes 集群时，可使用已发布的 OCI Helm Chart `oci://ghcr.io/multica-ai/charts/multica`，或源码中的 [`deploy/helm/multica/`](deploy/helm/multica/)，代替 Docker Compose 部署。Chart 面向具有 Ingress 控制器和默认 `ReadWriteOnce` StorageClass 的常见 k3s／k8s 环境，基于 k3s、Traefik 和 `local-path` 编写，其他集群通常只需少量调整。

Chart 会在目标命名空间创建以下资源：

- `multica-postgres`：使用 `pgvector/pgvector:pg17`，配备 10Gi PVC。
- `multica-backend`：Go API／WebSocket 服务，默认配备 5Gi `ReadWriteOnce` 上传文件 PVC。已配置 S3（`backend.config.s3Bucket`）且不希望创建此 PVC 时，设置 `backend.uploads.persistence.enabled=false`。
- `multica-frontend`：Next.js standalone 服务。
- 两个 `Ingress` 资源，分别对应 Web 主机和后端主机。
- 根据 `values.yaml` 渲染的 `multica-config` ConfigMap。

`multica-secrets` Secret **不由 Chart 管理**。使用 `kubectl` 创建一次，避免真实密钥进入 Git。

> **前端运行时上游：** 当前 `multica-web` 镜像在 Next.js 服务运行时读取 `REMOTE_API_URL` 和 `DOCS_URL`，修改 API 或文档上游地址无需重新构建前端。Chart 默认将 `REMOTE_API_URL` 设为当前发布实例的后端 Service。`frontend.compatibility.backendAlias` 仅用于仍在构建时写死 `REMOTE_API_URL=http://backend:8080` 的旧镜像。

> **前置条件：** `kubectl` 和 `helm` 已配置为访问目标集群，Helm 使用支持 `--take-ownership` 的 v3.13+ 或 v4+；集群具有 Ingress 控制器（Traefik／NGINX）和默认 StorageClass。

<a id="step-1--point-hostnames-at-the-cluster"></a>

### 步骤 1 将域名指向集群

默认 Web 域名为 `multica.dev.lan`，后端为 `api.multica.dev.lan`。任选一种解析方式：

- 在所有需要访问的机器上配置 **`/etc/hosts`**，包括开发电脑和运行守护进程的机器：

  ```text
  192.168.1.206  multica.dev.lan api.multica.dev.lan
  ```

  将 `192.168.1.206` 替换为可访问 Ingress 控制器 Service 的任一集群节点 IP。

- 使用 **本地 DNS**，例如 Pi-hole、Unbound，为两个域名添加指向集群 Ingress IP 的 A 记录。

需要其他域名时，在安装时覆盖对应值，见[步骤 4](#step-4--install-the-chart)：`ingress.frontend.host`、`ingress.backend.host`，以及 `backend.config.appUrl`、`backend.config.frontendOrigin`、`backend.config.localUploadBaseUrl` 和 `backend.config.googleRedirectUri`。

<a id="step-2--create-the-namespace"></a>

### 步骤 2 创建命名空间

```bash
kubectl create namespace multica
```

<a id="step-3--create-the-multica-secrets-secret"></a>

### 步骤 3 创建 multica-secrets Secret

Chart 按名称引用此 Secret，使用随机值创建一次：

```bash
kubectl -n multica create secret generic multica-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -hex 32)" \
  --from-literal=POSTGRES_PASSWORD="$(openssl rand -hex 16)" \
  --from-literal=RESEND_API_KEY="" \
  --from-literal=GOOGLE_CLIENT_SECRET="" \
  --from-literal=CLOUDFRONT_PRIVATE_KEY="" \
  --from-literal=MULTICA_DEV_VERIFICATION_CODE=""
```

可选值暂时留空，之后再补充，见[步骤 5 登录](#step-5--log-in)。

<a id="step-4--install-the-chart"></a>

### 步骤 4 安装 Chart

```bash
helm install multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica
```

Chart 版本使用去掉前缀 `v` 的 Git 标签。例如发布标签 `v0.3.5` 对应 Chart 版本 `0.3.5`，后端和前端镜像默认使用 `v0.3.5` 标签。

覆盖默认配置时，先导出 Chart 配置、编辑文件，再用 `-f` 传入：

```bash
helm show values oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> > my-values.yaml
# 编辑 my-values.yaml，例如修改 Ingress 域名、镜像标签和资源限制
helm install multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

从本地检出代码开发时，改用本地 Chart 路径：

```bash
helm install multica deploy/helm/multica -n multica
```

后端需要访问由内部 CA 签发证书的服务（例如自托管 Gitea）时，配置 `backend.extraCACerts.configMap`。见[高级配置中的私有 CA 证书](SELF_HOSTING_ADVANCED.zh.md#private-ca-certificates-optional)。

观察 Pod 启动：

```bash
kubectl -n multica get pods -w
```

首次启动的集群中，后端等待 PostgreSQL 和执行迁移时，可能在数分钟内处于 `Running` 但尚未 `Ready` 的状态。startupProbe 会覆盖这段启动时间，Pod 不应因此重启。后端报告 `Ready` 后，表示迁移已完成，`/healthz` 返回正常状态：

```bash
curl -H "Host: api.multica.dev.lan" http://<ingress-ip>/healthz
# {"status":"ok","checks":{"db":"ok","migrations":"ok"}}
```

随后在浏览器中打开 http://multica.dev.lan。

<a id="step-5--log-in"></a>

### 步骤 5 登录

Chart 默认使用 `APP_ENV=production`，配置位于 `values.yaml` 的 `backend.config.appEnv`，默认没有固定验证码。与 Docker 部署相同，可选择以下三种方式：

- **生产环境推荐：** 将真实 Resend 密钥写入 Secret，然后重启后端：

  ```bash
  kubectl -n multica patch secret multica-secrets --type=merge \
    -p '{"stringData":{"RESEND_API_KEY":"re_xxx"}}'
  kubectl -n multica rollout restart deploy/multica-backend
  ```

  系统会向输入的邮箱发送真实验证码。见[高级配置中的邮件设置](SELF_HOSTING_ADVANCED.zh.md#email-required-for-authentication)。

- **未配置邮件：** 服务端生成验证码并输出到后端 Pod 日志，可搜索 `[DEV] Verification code for ...:`，适合临时测试。

  ```bash
  kubectl -n multica logs -f deploy/multica-backend | grep "Verification code"
  ```

- **固定验证码的本地或私有测试：** 在配置文件中设置 `backend.config.appEnv: development`，在 Secret 中设置 `MULTICA_DEV_VERIFICATION_CODE=888888`，然后执行 `helm upgrade` 并重启。`APP_ENV=production` 时忽略固定验证码。

  ```bash
  helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
    --version <chart-version> \
    -n multica \
    -f my-values.yaml --set backend.config.appEnv=development
  kubectl -n multica patch secret multica-secrets --type=merge \
    -p '{"stringData":{"MULTICA_DEV_VERIFICATION_CODE":"888888"}}'
  kubectl -n multica rollout restart deploy/multica-backend
  ```

`ALLOW_SIGNUP`、`DISABLE_WORKSPACE_CREATION`、`WORKSPACE_CREATION_ALLOWED_EMAILS` 和 `GOOGLE_CLIENT_ID` 同样配置在 `values.yaml` 的 `backend.config.*` 中，名称依次为 `allowSignup`、`disableWorkspaceCreation`、`workspaceCreationAllowedEmails` 和 `googleClientId`。执行 `helm upgrade` 后，ConfigMap 哈希变化会触发后端 Pod 自动滚动更新。Web 界面在运行时从 `/api/config` 读取 `allowSignup`、`disableWorkspaceCreation` 和 `googleClientId`，无需重新构建前端。`workspaceCreationAllowedEmails` 仅由后端读取，不会通过 `/api/config` 暴露。

> **警告：** 不要在可公开访问的实例上设置 `MULTICA_DEV_VERIFICATION_CODE`，否则知道邮箱地址的人都能使用固定验证码登录。

<a id="step-6--install-cli--start-daemon"></a>

### 步骤 6 安装 CLI 并启动守护进程

守护进程运行在本机，不在集群内。按上方[步骤 3](#step-3--install-cli--start-daemon)安装 CLI 和 AI 智能体，再将 CLI 指向 Ingress 域名：

```bash
multica setup self-host \
  --server-url http://api.multica.dev.lan \
  --app-url http://multica.dev.lan
```

确认运行守护进程的机器也具有[步骤 1](#step-1--point-hostnames-at-the-cluster)中的 `/etc/hosts` 或 DNS 记录。

<a id="updating"></a>

### 更新

配置仍使用可变的 `latest` 镜像标签时，可以不修改 Chart 版本，直接重新部署以拉取最新镜像：

```bash
kubectl -n multica rollout restart deploy/multica-backend deploy/multica-frontend
```

升级到指定 Multica 版本时，使用对应的 Chart 版本。已发布 Chart 默认使用匹配 Git 标签的应用镜像：

```bash
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

需要独立于 Chart 版本指定应用镜像时，在配置文件中设置镜像标签：

```yaml
images:
  backend:
    tag: v0.2.4
  frontend:
    tag: v0.2.4
```

然后使用相同的升级命令，通过 `-f my-values.yaml` 传入配置：

```bash
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> \
  -n multica \
  -f my-values.yaml
```

升级异常时可以回滚：

```bash
helm -n multica rollback multica
```

> **从 `v0.3.4` 升级到 `v0.3.5+` 时出现 `refusing to drop legacy daily rollups: ...`？** 从 MUL-2957 起，`migrate up` 会在应用迁移 `103` 前自动执行按月分片的幂等回填，正常升级只需一次 `helm upgrade` 和后端滚动更新。如果仍使用 MUL-2957 之前的二进制程序，或自动钩子失败，请对 Chart 使用的同一数据库运行独立回填命令 `kubectl -n multica exec deploy/multica-backend -- ./backfill_task_usage_hourly --sleep-between-slices=2s`，然后重启后端重新执行迁移。完整恢复步骤见[高级配置中的用量仪表盘汇总](SELF_HOSTING_ADVANCED.zh.md#usage-dashboard-rollup)。

<a id="tearing-down"></a>

### 卸载

```bash
# 移除工作负载，保留 PVC 和 Secret
helm -n multica uninstall multica

# 删除全部资源，包括 PostgreSQL 数据和上传文件
kubectl delete namespace multica
```

---

<a id="usage-dashboard-rollup"></a>

## 用量仪表盘汇总

用量和运行时仪表盘读取派生表 `task_usage_hourly`，由 `rollup_task_usage_hourly()` 填充。从 MUL-2957 起，各后端副本通过数据库支持的调度器 `sys_cron_executions` 在**进程内**执行汇总。全新自托管安装无需额外操作，附带的 `pgvector/pgvector:pg17` 镜像可直接使用；无需更换为包含 `pg_cron` 的镜像、注册外部 cron、配置 systemd 定时器或 Kubernetes `CronJob`。

多个后端副本可以安全运行：每个副本每 30 秒尝试领取当前 5 分钟 UTC 计划，唯一键 `(job_name, scope_kind, scope_id, plan_time)` 保证每个计划只有一个副本成功领取。可以通过下方查询检查运行情况。

> **企业微信智能机器人与副本数量。** Slack 和飞书通过无状态 HTTP 发送消息，任意副本都可执行；企业微信智能机器人只能通过进程内的 WebSocket 长连接发送，由持有该机器人租约的副本维护。其他副本产生回复后的处理取决于实时 relay 模式：
>
> - **分片或 dual relay 模式（已设置 `REDIS_URL`，使用 Redis 时的默认模式）：** 回复或收件箱推送通过 relay 转发给租约持有者并发送，此模式支持企业微信多副本部署。
> - **legacy relay 模式或未配置 Redis：** 回复会被丢弃，企业微信用户收不到消息。此配置下，启用企业微信的后端应使用单副本。
>
> 所有模式仍存在一个窗口：所有副本都在重连、没有任何副本持有机器人有效连接时，产生的回复无法交付。系统会统计此情况：发起路由的副本事后检查是否有副本领取过发送任务，没有时增加 `multica_wecom_outbound_dropped_total{reason="no_live_connection"}`。运维可据此测量该窗口。若不能接受重连期间偶发丢失回复，单副本仍是更保守的部署方式。其他部分，包括上述汇总调度器，都支持多副本安全运行。

```sql
SELECT plan_time, status, attempt, runner_id,
       error_code, error_msg, started_at, finished_at
  FROM sys_cron_executions
 WHERE job_name = 'rollup_task_usage_hourly'
 ORDER BY plan_time DESC
 LIMIT 20;
```

审计表语义、advisory lock 4246、独立回填命令、参数说明以及 `v0.3.4 → v0.3.5+` 自动迁移钩子的完整说明，见[高级配置中的用量仪表盘汇总](SELF_HOSTING_ADVANCED.zh.md#usage-dashboard-rollup)。

> **从 `v0.3.4` 升级到 `v0.3.5+`？** 从 MUL-2957 起，`migrate up` 会在迁移 `103` 前自动执行按月分片的幂等回填，一次调用即可完成升级，无需额外运维步骤。旧二进制程序或环境问题导致自动钩子失败时，请对同一数据库运行 `backfill_task_usage_hourly`，然后重新升级。恢复步骤见[高级配置中的用量仪表盘汇总](SELF_HOSTING_ADVANCED.zh.md#usage-dashboard-rollup)。

<a id="compatibility-paths-existing-deployments-only"></a>

### 兼容方式 仅适用于已有部署

在 MUL-2957 之前，直接调用 `SELECT rollup_task_usage_hourly()` 的外部调度器是唯一方式，包括数据库中注册的 **`pg_cron`、外部 cron、systemd 定时器和 Kubernetes `CronJob`**。这些方式仍受支持，但新部署应使用进程内调度器。SQL 函数内部持有 advisory lock 4246，因此原有外部调度与进程内调度可以共存，不会重复写入汇总。

已有生产 `pg_cron` 作业时，建议按以下顺序停用：

1. 确认至少一个后端副本的进程内调度器健康，`sys_cron_executions` 中持续出现 `rollup_task_usage_hourly` 的近期 SUCCESS 记录：

   ```sql
   SELECT plan_time, status, runner_id, finished_at
     FROM sys_cron_executions
    WHERE job_name = 'rollup_task_usage_hourly'
      AND status = 'SUCCESS'
    ORDER BY plan_time DESC
    LIMIT 5;
   ```

2. 确认 SUCCESS 记录按计划持续出现后，取消冗余的 `pg_cron` 作业：

   ```sql
   SELECT cron.unschedule('rollup_task_usage_hourly')
     FROM cron.job WHERE jobname = 'rollup_task_usage_hourly';
   ```

3. 除非确认没有其他工作负载依赖，否则保留 `pg_cron` 扩展。附带的 `pgvector/pgvector:pg17` 镜像不包含 `pg_cron`，Multica 默认安装也不依赖它；从其他工作负载仍在使用的自定义镜像中移除该扩展，需要单独评估。

直接调用 `SELECT rollup_task_usage_hourly()` 的外部 cron、systemd 定时器和 Kubernetes `CronJob` 也可按同样方式停用：确认进程内调度器持续产生 SUCCESS 记录后，外部作业已无必要，可以移除。

<a id="stopping-services"></a>

## 停止服务

通过安装脚本部署时：

```bash
curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash -s -- --stop
```

手动克隆仓库时：

```bash
# 停止 Docker Compose 服务，包括后端、前端和数据库
make selfhost-stop

# 停止本地守护进程
multica daemon stop
```

<a id="switching-to-multica-cloud"></a>

## 切换到 Multica Cloud

从自托管切换 CLI 到 [Multica Cloud](https://multica.ai) 时运行：

```bash
multica setup
```

命令会将 CLI 重新配置为连接 multica.ai，重新认证并重启守护进程。覆盖现有配置前会提示确认。

> 本地 Docker 服务不受影响。不再使用时，请单独停止。

<a id="upgrading"></a>

## 升级

```bash
docker compose -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.selfhost.yml up -d
```

需要固定版本时，在 `.env` 中将 `MULTICA_IMAGE_TAG` 设为 `v0.2.4` 等准确版本。后端启动时会自动运行迁移。
如果所选 GHCR 标签尚未发布，改用 `make selfhost-build`，或运行 `docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build`。

> **从 `v0.3.4` 升级到 `v0.3.5+` 时出现 `refusing to drop legacy daily rollups: ...`？** 这是迁移 `103` 的保护机制：只有先填充 `task_usage_hourly`，才允许删除旧的每日汇总。从 MUL-2957 起，`migrate up` 会在迁移 `103` 前自动回填，一次调用即可完成升级。使用旧二进制程序或环境问题导致自动钩子失败时，应先手动运行 `backfill_task_usage_hourly`，再重新升级。完整步骤见[高级配置中的用量仪表盘汇总](SELF_HOSTING_ADVANCED.zh.md#usage-dashboard-rollup)。

---

<a id="manual-docker-compose-setup"></a>

## 手动 Docker Compose 部署

如需自行执行 Docker Compose 步骤，可代替 `make selfhost`：

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
cp .env.example .env
```

编辑 `.env`，设置必填的 `JWT_SECRET`。该值为空时 Docker Compose 会拒绝启动；生产后端也会拒绝开发默认值或任何已知占位值。

```bash
JWT_SECRET=$(openssl rand -hex 32)
```

然后启动所有服务：

```bash
docker compose -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.selfhost.yml up -d
```

<a id="manual-cli-configuration"></a>

## 手动配置 CLI

如需逐项配置 CLI，可代替 `multica setup`：

```bash
# 将 CLI 指向本地服务端
multica config set server_url http://localhost:8080
multica config set app_url http://localhost:3000

# 登录，会打开浏览器
multica login

# 启动守护进程
multica daemon start
```

使用 TLS 的生产部署：

```bash
multica config set app_url https://app.example.com
multica config set server_url https://api.example.com
multica login
multica daemon start
```

<a id="advanced-configuration"></a>

## 高级配置

环境变量、不使用 Docker 的手动部署、反向代理、数据库设置等说明，见[自托管高级配置](SELF_HOSTING_ADVANCED.zh.md)。
