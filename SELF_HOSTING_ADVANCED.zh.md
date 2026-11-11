<a id="self-hosting--advanced-configuration"></a>

# 自托管高级配置

[English](SELF_HOSTING_ADVANCED.md) | 简体中文

本文介绍 Multica 自托管部署的高级配置。快速上手见[自托管指南](SELF_HOSTING.zh.md)。

<a id="configuration"></a>

## 配置

所有配置都通过环境变量传入，可以复制 `.env.example` 作为起点。

<a id="required-variables"></a>

### 必需变量

| 变量 | 说明 | 示例 |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL 连接字符串 | `postgres://multica:multica@localhost:5432/multica?sslmode=disable` |
| `JWT_SECRET` | **必填，没有安全的默认值。** 用于签发 JWT。生产后端在值为空或使用已知占位值时会拒绝启动。使用 `openssl rand -hex 32` 生成 | `openssl rand -hex 32` |
| `FRONTEND_ORIGIN` | 前端访问 URL，用于 CORS | `https://app.example.com` |

<a id="database-pool-tuning-optional"></a>

### 数据库连接池调优 可选

默认配置适用于常见部署，仅在规模较大或资源受限时需要调优。每个连接池的优先级依次为：对应环境变量 → 对应 URL 中的 `pool_*` 查询参数 → 内置默认值。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `DATABASE_MAX_CONNS` | 每个 Pod 的 pgxpool 最大连接数。`pod_count × DATABASE_MAX_CONNS` 应明显低于 PostgreSQL 的 `max_connections` 上限。前方有 PgBouncer／RDS Proxy／Supavisor 等连接池代理时，可显著提高 | `25` |
| `DATABASE_MIN_CONNS` | 每个 Pod 保持预热的基线连接数，自动限制为不超过 `DATABASE_MAX_CONNS` | `5` |
| `DATABASE_REPLICA_URL` | 可选的 PostgreSQL 只读副本连接字符串。新连接会验证只读状态，只有代码中的读取路径显式选择副本后，业务流量才会使用 | - |
| `DATABASE_REPLICA_MAX_CONNS` | 每个 Pod 的副本最大连接数，独立于 `DATABASE_MAX_CONNS` | `10` |
| `DATABASE_REPLICA_MIN_CONNS` | 每个 Pod 保持预热的副本连接数 | `0` |

应将所有 API Pod 的主库和副本连接池上限合计，与 PostgreSQL 集群连接上限一起规划。副本连接默认存活 5 分钟，可通过 URL 参数 `pool_max_conn_lifetime` 覆盖，以便节点提升后重新验证只读状态。副本属于非关键配置：配置无效或运行时连接失败时，已选择副本的读取会回退到主库，并通过短暂的被动熔断避免每个请求都等待连接超时。现有读取路径在逐项迁移前仍只使用主库。应用不会强制限制副本数据的陈旧程度，应在数据库层监控复制延迟，并将一致性敏感读取保留在主库。

<a id="email-required-for-authentication"></a>

### 邮件 认证所需

Multica 支持两种邮件后端。设置了 `SMTP_HOST` 时优先使用 SMTP，否则使用 `RESEND_API_KEY`。两者都未配置时，验证码输出到服务端日志，可从日志复制后登录。

<a id="option-a-resend-recommended-for-cloud-deployments"></a>

#### 方案 A Resend 推荐用于云部署

| 变量 | 说明 |
| --- | --- |
| `RESEND_API_KEY` | Resend API 密钥 |
| `RESEND_FROM_EMAIL` | 发件邮箱，默认为 `noreply@multica.ai` |

<a id="option-b-smtp-relay-for-self-hosted--on-premise-deployments"></a>

#### 方案 B SMTP 中继 适用于自托管或本地部署

部署无法访问公网，或已有 Exchange、Postfix、本地 SendGrid 等内部邮件中继时，可使用此方式。

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `SMTP_HOST` | SMTP 中继主机名，配置后启用 SMTP 模式 | - |
| `SMTP_PORT` | SMTP 端口 | `25` |
| `SMTP_USERNAME` | SMTP 用户名，无认证中继可留空 | - |
| `SMTP_PASSWORD` | SMTP 密码 | - |
| `SMTP_TLS` | TLS 模式。`implicit`，别名为 `smtps`、`ssl`，在连接时直接使用 SMTPS，端口 `465` 会自动启用；留空或 `starttls` 时通过 STARTTLS 升级 | `starttls` |
| `SMTP_TLS_INSECURE` | 设为 `true` 时跳过 TLS 证书验证，用于自签名或私有 CA 证书 | `false` |
| `SMTP_EHLO_NAME` | 向中继声明的 EHLO／HELO 名称。严格中继（如 Google Workspace）拒绝来自公网 IP 的默认问候名称时，应配置真实 FQDN | 本机主机名 |

服务端声明支持 STARTTLS 时，会自动使用。支持 465 端口的 SMTPS／隐式 TLS，且该端口会自动启用隐式 TLS；在非标准端口上可设置 `SMTP_TLS=implicit`，别名为 `smtps`、`ssl`，强制使用。

> **说明：** Resend 和 SMTP 都未配置时，验证码输出到后端日志，可从日志复制后登录。固定的本地测试验证码（如 `888888`）需要显式启用：在 `.env` 中设置 `MULTICA_DEV_VERIFICATION_CODE=888888`，并保持 `APP_ENV` 为非生产环境。Docker 自托管默认固定为 `APP_ENV=production`，会忽略此快捷配置。**不要在可公开访问的实例上启用固定验证码。**

<a id="google-oauth-optional"></a>

### Google OAuth 可选

| 变量 | 说明 |
| --- | --- |
| `GOOGLE_CLIENT_ID` | Google OAuth 客户端 ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 客户端密钥 |
| `GOOGLE_REDIRECT_URI` | OAuth 回调 URL，例如 `https://app.example.com/auth/callback` |

修改后重启后端或 Compose 服务即可生效。Web 界面在运行时从 `/api/config` 读取 `GOOGLE_CLIENT_ID`，无需重新构建前端。

<a id="signup-controls-optional"></a>

### 注册控制 可选

| 变量 | 说明 |
| --- | --- |
| `ALLOW_SIGNUP` | 设为 `false` 时，仅允许白名单或受邀用户注册新账号 |
| `ALLOWED_EMAIL_DOMAINS` | 可选的邮箱域名白名单，以逗号分隔 |
| `ALLOWED_EMAILS` | 可选的完整邮箱地址白名单，以逗号分隔 |
| `DISABLE_WORKSPACE_CREATION` | 设为 `true` 时，所有调用方访问 `POST /api/workspaces` 都返回 403，用户只能加入受邀工作区 |
| `WORKSPACE_CREATION_ALLOWED_EMAILS` | 可选的工作区创建者完整邮箱地址列表，以逗号分隔。配置后其他人调用 `POST /api/workspaces` 返回 403。`DISABLE_WORKSPACE_CREATION=true` 时忽略此列表 |

修改后重启后端或 Compose 服务即可生效。Web 界面在运行时从 `/api/config` 读取 `ALLOW_SIGNUP` 和 `DISABLE_WORKSPACE_CREATION`，无需重新构建前端。

<a id="locking-down-workspace-creation"></a>

#### 禁止继续创建工作区

`ALLOW_SIGNUP=false` 限制新账号注册，但已登录用户仍可通过 `POST /api/workspaces` 创建其他工作区。如果自托管实例要求平台管理员能看到所有任务、仓库和智能体，应设置 `DISABLE_WORKSPACE_CREATION=true`。推荐初始化顺序如下：

1. 使用默认的 `DISABLE_WORKSPACE_CREATION=false` 启动实例。
2. 以管理员身份登录并创建共享工作区。
3. 设置 `DISABLE_WORKSPACE_CREATION=true` 并重启后端。需要限制新账号注册时，可同时设置 `ALLOW_SIGNUP=false`。
4. 此后新增用户只能通过邀请加入，界面隐藏创建工作区入口，直接调用 API 也返回 403。

<a id="letting-only-selected-users-create-workspaces"></a>

#### 仅允许指定用户创建工作区

少数用户（如平台或团队负责人）仍需创建工作区，其他用户仅通过邀请加入时，可以设置 `WORKSPACE_CREATION_ALLOWED_EMAILS`，避免反复切换 `DISABLE_WORKSPACE_CREATION`：

```dotenv
WORKSPACE_CREATION_ALLOWED_EMAILS=alice@example.com,bob@example.com
```

`POST /api/workspaces` 按以下顺序判断：

1. `DISABLE_WORKSPACE_CREATION=true`：所有人返回 403，包括名单中的用户。
2. `WORKSPACE_CREATION_ALLOWED_EMAILS` 为空：允许所有已登录用户创建，这是默认行为。
3. 调用方账号邮箱与列表中的地址匹配，不区分大小写：允许创建。
4. 其他情况：返回 403。

还需注意：

- 此列表只控制创建工作区的权限，不代表实例管理员角色。名单中的用户不会获得其他工作区的查看或管理权限；实例运营人员需要查看新工作区时，仍需被邀请加入。
- 界面不会为名单外用户隐藏创建工作区入口，他们尝试创建时会看到服务端错误。
- 此列表独立于仅控制注册权限的 `ALLOWED_EMAILS`／`ALLOWED_EMAIL_DOMAINS`。

> **说明：** `ALLOW_SIGNUP=false` 启用仅受邀注册。持有有效待处理工作区邀请的新用户，可使用受邀邮箱创建账号；既不匹配白名单也没有有效邀请的用户仍会被拒绝。`ALLOW_SIGNUP=true` 时，邀请也允许白名单外的邮箱注册。撤销邀请不会删除已通过邀请创建的账号，也不会阻止其登录。要求受邀用户只能加入现有工作区时，应同时设置 `DISABLE_WORKSPACE_CREATION=true`。

<a id="file-storage-optional"></a>

### 文件存储 可选

上传文件和附件默认写入本地磁盘。配置 `S3_BUCKET` 后改用 S3 兼容对象存储。

<a id="local-disk-default"></a>

#### 本地磁盘 默认方式

| 变量 | 说明 |
| --- | --- |
| `LOCAL_UPLOAD_DIR` | 附件写入目录，默认为 `./data/uploads`。该路径**相对于后端工作目录**：附带镜像中的目录为 `/app`，Compose 会将 `backend_uploads` 卷挂载到对应位置。手动运行二进制程序时，应设为绝对路径；否则启动目录改变后，文件会写到新位置，已有 `attachment` 记录对应的文件也可能无法找到 |
| `LOCAL_UPLOAD_BASE_URL` | 可选的附件绝对基础 URL，例如 `http://localhost:8080`。留空时保存 `/uploads/...` 路径，始终相对于 API 来源解析 |

上传支持最大 **100 MiB** 的文件，后端额外允许 1 MiB 用于 multipart 边界和元数据，超限文件或请求返回 HTTP 413。Web 上传路由会将请求体流式转发给后端，因此即使请求没有 `Content-Length`，也不会被 Next Proxy 的请求体缓冲上限截断。使用自己的反向代理或 Ingress 时，也应允许至少 **101 MiB** 的请求体。上传上限固定，不提供上传大小环境变量。

<a id="s3--cloudfront"></a>

#### S3 与 CloudFront

| 变量 | 说明 |
| --- | --- |
| `S3_BUCKET` | 仅填写存储桶名称，例如 `my-bucket`，不要包含 `.s3.<region>.amazonaws.com` 后缀；服务端使用 `S3_BUCKET` 和 `S3_REGION` 构建公开 URL |
| `S3_REGION` | AWS 区域，默认为 `us-west-2`，必须与存储桶真实区域一致，用于 SDK 签名和公开 URL |
| `AWS_ACCESS_KEY_ID`／`AWS_SECRET_ACCESS_KEY` | 静态凭据，两者都未设置时使用 AWS SDK 默认凭据链 |
| `AWS_ENDPOINT_URL` | 自定义 S3 兼容端点，例如 MinIO、R2、B2。为保持兼容，设置后默认使用路径形式 URL |
| `S3_USE_PATH_STYLE` | 可选的 S3 寻址模式。留空时，配置 `AWS_ENDPOINT_URL` 的默认值为 `true`，AWS S3 为 `false`。服务商要求虚拟主机形式 URL 时设为 `false` |
| `ATTACHMENT_DOWNLOAD_MODE` | 附件下载方式：`auto`（默认）、`cloudfront`、`presign` 或 `proxy`。使用 `http://rustfs:9000` 等仅 Docker／VPC 内可访问端点的私有桶时，选择 `proxy`。头像遵循同一策略，见下文 |
| `ATTACHMENT_DOWNLOAD_URL_TTL` | CloudFront 签名 URL 和 S3 预签名下载 URL 的有效期，默认为 `30m` |
| `CLOUDFRONT_DOMAIN` | CloudFront 分发域名，配置后公开 URL 使用该主机代替 S3 主机 |
| `CLOUDFRONT_KEY_PAIR_ID` | 签名 URL 使用的 CloudFront 密钥对 ID |
| `CLOUDFRONT_PRIVATE_KEY` | PEM 格式的 CloudFront 私钥 |

<a id="avatars-on-a-private-bucket"></a>

#### 私有存储桶中的头像

用户、智能体、小队和工作区头像保存为原始存储对象 URL。存储桶公开时，包括公开的 `CLOUDFRONT_DOMAIN` 或默认本地磁盘存储，该 URL 会原样返回给客户端。

存储桶私有且没有配置公开 CDN 域名时，例如开启 Block Public Access 的 S3、R2 或 MinIO，API 通过 `/api/avatars/<signature>/<key>` 提供头像，并按 `ATTACHMENT_DOWNLOAD_MODE` 对每次请求使用预签名重定向、CloudFront 签名重定向或代理响应体。路径中的签名用于授权读取：会话 Cookie 使用 `SameSite=Strict`，因此需要认证的 URL 无法直接作为桌面端或分离来源前端的 `<img src>`。

此路由仅处理头像类别的图片，即没有关联任务、评论、聊天或运行的独立上传图片。将 `avatar_url` 指向任务附件会在设置时被拒绝；已经保存的这类值返回 404，防止通过头像字段将私有图片变成公开链接。

无需额外配置，在此行为引入前上传的头像也会自动适配，无需回填。

<a id="cookies"></a>

### Cookie

| 变量 | 说明 |
| --- | --- |
| `COOKIE_DOMAIN` | 会话和 CloudFront Cookie 的可选 `Domain` 属性。单主机部署（localhost、局域网 IP 或单一域名）应**留空**。仅当前后端使用同一注册域名下的不同子域名时配置，例如 `.example.com`。**不要使用 IP 字面量**，RFC 6265 禁止 Cookie 的 `Domain` 使用 IP 地址，浏览器会丢弃此类 `Set-Cookie` 响应头 |

会话 Cookie 的 `Secure` 标记自动根据 `FRONTEND_ORIGIN` 协议推导：HTTPS 使用安全 Cookie，局域网或私有网络中的普通 HTTP 使用非安全 Cookie，使浏览器能够正常保存。

前后端使用不同主机名时必须配置 `COOKIE_DOMAIN`：浏览器需要在页面自身来源读取 `multica_csrf` Cookie，以发送 `X-CSRF-Token` 请求头，否则所有写请求都会返回 `403 {"error":"CSRF validation failed"}`。请使用覆盖两个主机的最窄父域名，因为 `multica_auth` 会话 Cookie 也会共享给该域名下所有主机。完整分域配置及信任要求见[反向代理](#reverse-proxy)。

<a id="server"></a>

### 服务端

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 通常只需修改此后端端口。本地直接运行时为进程监听端口，Compose 自托管时为宿主机发布端口。Compose 容器内部始终监听 `8080`，修改无需重新构建 |
| `BACKEND_PORT` | `PORT` 的值 | 覆盖后端 `PORT` 的可选别名，另有 `API_PORT` 和 `SERVER_PORT`。优先级为 `BACKEND_PORT` → `API_PORT` → `SERVER_PORT` → `PORT` → `8080`，在 `Makefile`、`scripts/local-env.sh` 和 `docker-compose.selfhost.yml` 中一致。仅在宿主机端口需要与进程监听端口不同时配置 |
| `METRICS_ADDR` | 空 | 可选的 Prometheus 指标监听地址，例如 `127.0.0.1:9090` |
| `FRONTEND_PORT` | `3000` | 前端端口，在 Compose 中表示宿主机端口，容器内部始终监听 `3000` |
| `CORS_ALLOWED_ORIGINS` | `FRONTEND_ORIGIN` 的值 | 以逗号分隔的允许来源列表，同时控制 HTTP CORS 和 WebSocket `Origin` 检查。未列入列表且不是 localhost 的浏览器来源会收到 WebSocket 升级 403，实时更新停止，只能手动刷新 |
| `LOG_LEVEL` | `info` | 日志级别：`debug`、`info`、`warn`、`error` |

> **配置来源优先级取决于启动入口**，只有上述别名顺序是共用的。Docker Compose 中，调用环境优先于 `.env`，例如 `PORT=9100 docker compose … up -d` 会发布 9100。`make` 会通过 `include` 读取环境文件，因此文件值优先于环境变量，而 `make selfhost PORT=…` 形式的命令行赋值优先于两者。直接修改环境文件是在各入口下行为一致的方式。无论优先级如何，`make selfhost` 和两个安装器都会从 `docker compose port` 回读已发布端口，健康检查和输出 URL 始终匹配 Compose 实际发布的端口。

Web 开发服务器有意不使用通用 `PORT` 作为后端回退值：Next 在求值重写配置前，会将 `PORT` 用作自己的前端监听端口。因此，开发服务器的后端回退优先级为 `BACKEND_PORT` → `API_PORT` → `SERVER_PORT` → `8080`；显式的 `REMOTE_API_URL` 或 `NEXT_PUBLIC_API_URL` 仍然优先。

<a id="private-ca-certificates-optional"></a>

### 私有 CA 证书 可选

后端使用镜像中的系统信任库验证所调用 HTTPS 服务的 TLS 证书。访问内部 CA 签发证书的服务，例如[自托管 Gitea、Forgejo 或 GitLab](https://multica.ai/docs/vcs-integration)，需要先让后端信任该 CA，否则连接 Git 服务时会提示证书签发机构不受信任。

应将 CA 加入后端信任库。后端是 Go 程序，在 Linux 上加载系统证书包，以及 `SSL_CERT_DIR` 所列目录中的所有 PEM 文件。将 CA 挂载到单独的只读目录，并在系统目录后列出：

```
SSL_CERT_DIR=/etc/ssl/certs:/etc/multica/ca-certs
```

**Kubernetes／Helm：** 创建以 PEM 文件为键的 ConfigMap，将 `backend.extraCACerts.configMap` 指向该对象。Chart 会把它挂载到 `/etc/multica/ca-certs`，并按上述方式设置 `SSL_CERT_DIR`。

```bash
kubectl -n multica create configmap multica-extra-ca --from-file=internal-ca.crt
helm upgrade multica oci://ghcr.io/multica-ai/charts/multica \
  --version <chart-version> -n multica --reuse-values \
  --set backend.extraCACerts.configMap=multica-extra-ca
```

**Docker Compose：** 将 PEM 文件放在 `docker-compose.selfhost.yml` 旁的目录中，例如 `./ca-certs`，然后增加覆盖文件 `docker-compose.ca.yml`：

```yaml
services:
  backend:
    environment:
      SSL_CERT_DIR: /etc/ssl/certs:/etc/multica/ca-certs
    volumes:
      - ./ca-certs:/etc/multica/ca-certs:ro
```

```bash
docker compose -f docker-compose.selfhost.yml -f docker-compose.ca.yml up -d backend
```

每次执行可能重建后端的命令时，都传入这两个文件。只使用 `docker-compose.selfhost.yml` 的命令（如 `make selfhost`）会重建不包含 CA 配置的后端。

还需注意：

- **后端仅在启动时读取 CA。** 添加或替换 CA 文件后，需要运行 `kubectl -n multica rollout restart deploy/multica-backend`，或 `docker compose -f docker-compose.selfhost.yml -f docker-compose.ca.yml restart backend`。Compose 配置未变时，单独执行 `up -d` 会保留当前容器，无法使证书重新加载。Helm 的 ConfigMap 变化后，`helm upgrade` 也会重启后端。
- **CA 对整个后端进程生效。** 所有使用系统信任库的出站 TLS 客户端都会信任它，不仅限于 Git 服务集成。
- **守护进程的 Git 也会获得 CA。** 守护进程使用本机安装的 `git` 克隆和拉取工作区仓库，无法直接读取后端的 `SSL_CERT_DIR`。因此后端会随工作区仓库信息发送新增目录中的证书，排除系统目录 `/etc/ssl/certs` 和 `/etc/pki/tls/certs`。守护进程将这些证书与本机 Git 已有信任合并，并在每个工作区仓库的 Git 配置中记录结果。守护进程的克隆、拉取，以及智能体在检出目录执行的 Git 命令都会读取该配置，无需在守护进程机器上安装证书。具体规则如下：
  - 保留 Git 原有信任。系统或全局 Git 配置设置了通用 `http.sslCAInfo` 时，合并该文件中的证书；否则与系统 CA 包合并。结果写入守护进程自己的文件，不修改系统信任库或原有 Git 配置。
  - 更窄范围的配置优先，且不会合并：主机专用的 `http.<url>.sslCAInfo`、仓库自身配置中的 `http.sslCAInfo` 和 `GIT_SSL_CAINFO`。
  - 每次检出前都会刷新工作区仓库，届时获取新增、替换或移除的 CA，以及本机信任变更，无需重启守护进程；后端仍需重启。某个 CA 存在期间创建的检出目录，在该 CA 移除后仍可继续工作。
  - 在这些仓库内，Git 对所有主机都信任后端提供的 CA，不仅限于需要它的主机。
  - Windows 守护进程不应用此机制。Git for Windows 默认使用 Windows 证书存储验证，请将 CA 加入该存储。
- **此配置只解决信任问题。** 过期证书或不匹配 URL 主机名的证书仍会被拒绝，需要修复证书本身。

<a id="wecom-frame-tracing"></a>

### 企业微信帧追踪

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MULTICA_WECOM_TRACE` | 空，关闭 | 设为 `1` 时记录后端读取和写入的每一帧企业微信消息，包括每条正文的前 120 个 Unicode 字符，其他值均关闭 |

仅在调试期间启用，结束后取消设置。启用前需了解：

- **配置在启动时读取，修改后需要重启后端。** 使用 `docker compose -f docker-compose.selfhost.yml up -d backend`，没有运行时开关。启用期间，后端每次启动都会输出警告。
- **任何能读取后端日志的人都能读取被追踪的消息文本。** 日志可见范围可能大于企业微信聊天本身，包括 `docker logs`、journald、日志收集器及其管理员。不会读取机器人密钥，消息文本中的 `token=`、`access_token=`、`code=` 参数会脱敏，但用户消息内容不会脱敏。**附件的 `Content-Disposition` 及解析出的文件名会原样记录**，最多 2048 个 Unicode 字符，并绕过脱敏器。该日志用于准确显示存储后端的文件名编码，脱敏或截断后的内容无法用于判断，也意味着附件文件名会按原始内容进入日志。
- **保留时长由日志系统决定。** 后端仅写入 stderr，不自行保存；追踪文本保留多久取决于 Docker 日志驱动或收集器配置。如果配置为永久保留，应在启用前作出判断。

每个出站帧产生两条共享同一 `seq` 的日志：写入前记录 `dir=out`，套接字响应后记录 `dir=out.done` 和 `ok=true`／`ok=false`。`seq` 表示写入顺序，这一对日志可以确认后端发送了什么及其顺序；只有尝试记录而没有对应结果，说明写操作尚未返回。

`ok=true` 仅表示帧已写入套接字。要判断企业微信是否接受，应将帧的 `req_id` 与对应的 `dir=in` 响应匹配，再检查 `errcode`；成功写入的帧也可能被平台拒绝。

<a id="cli--daemon"></a>

### CLI 与守护进程

以下配置在每位用户的机器上设置：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `MULTICA_SERVER_URL` | `ws://localhost:8080/ws` | 守护进程到服务端的 WebSocket URL |
| `MULTICA_APP_URL` | `http://localhost:3000` | CLI 登录流程使用的前端 URL |
| `MULTICA_DAEMON_POLL_INTERVAL` | `30s` | 补偿性运行轮询间隔，正常情况下 WebSocket 唤醒信号会更快交付工作 |
| `MULTICA_DAEMON_WS_CLAIM_POLL_INTERVAL` | `3m` | WebSocket 健康时的领取安全轮询上限，独立于 `MULTICA_DAEMON_POLL_INTERVAL`。向下抖动使正常轮询间隔为 `2m30s`–`2m45s`；旧服务端及领取结果不确定时仍使用常规轮询间隔 |
| `MULTICA_DAEMON_HEARTBEAT_INTERVAL` | `15s` | 心跳频率 |

浏览器登录回调仅接受 `localhost`、`127.0.0.1` 和 RFC 1918 IPv4 地址。如果路由检测选择了不支持的虚拟网卡地址，例如 `198.18.0.1`，登录会在打开浏览器前停止。浏览器与 CLI 在同一机器时，使用 `multica login --callback-host 127.0.0.1` 重试；浏览器在其他局域网机器时，使用 `--callback-host <CLI-machine-LAN-IPv4>`。仅能通过 SSH 访问的机器可使用令牌登录，或按输出的 SSH 隧道说明操作。登录页会提示无效回调链接，不会将其当成普通 Web 登录。

智能体专用覆盖配置：

| 变量 | 说明 |
| --- | --- |
| `MULTICA_CLAUDE_PATH` | `claude` 二进制程序的自定义路径 |
| `MULTICA_CLAUDE_MODEL` | 覆盖使用的 Claude 模型 |
| `MULTICA_CODEX_PATH` | `codex` 二进制程序的自定义路径 |
| `MULTICA_CODEX_MODEL` | 覆盖使用的 Codex 模型 |
| `MULTICA_COPILOT_PATH` | `copilot`，即 GitHub Copilot CLI 二进制程序的自定义路径 |
| `MULTICA_COPILOT_MODEL` | 覆盖 Copilot 模型。GitHub Copilot 根据账号权益路由模型，因此不一定接受此设置 |
| `MULTICA_OPENCODE_PATH` | `opencode` 二进制程序的自定义路径 |
| `MULTICA_OPENCODE_MODEL` | 覆盖使用的 OpenCode 模型 |
| `MULTICA_CODEARTS_PATH` | `codearts` 启动器或二进制程序的自定义路径 |
| `MULTICA_CODEARTS_MODEL` | 覆盖使用的 CodeArts 模型 |
| `MULTICA_OPENCLAW_PATH` | `openclaw` 二进制程序的自定义路径 |
| `MULTICA_OPENCLAW_MODEL` | 选择默认使用的已注册 OpenClaw 智能体 ID；该智能体的 `runtime_config.model_override` 会持续覆盖后续新领取运行使用的底层模型，直到修改或清空 |
| `MULTICA_OPENCLAW_CLI_TIMEOUT` | 准备运行时，每次 `openclaw config ...` 调用的截止时间，默认 30s，接受 `45s` 或 `45`。本地 CLI 启动较慢时可提高；守护进程也会读取 CLI 配置中的 `backends.openclaw.cli_timeout` |
| `MULTICA_HERMES_PATH` | `hermes` 二进制程序的自定义路径 |
| `MULTICA_HERMES_MODEL` | 覆盖使用的 Hermes 模型 |
| `MULTICA_PI_PATH` | `pi` 二进制程序的自定义路径 |
| `MULTICA_PI_MODEL` | 覆盖使用的 Pi 模型 |
| `MULTICA_CURSOR_PATH` | `cursor-agent` 二进制程序的自定义路径 |
| `MULTICA_CURSOR_MODEL` | 覆盖使用的 Cursor Agent 模型 |
| `MULTICA_GROK_PATH` | `grok` 二进制程序的自定义路径 |
| `MULTICA_GROK_MODEL` | 覆盖使用的 Grok 模型，例如 `grok-4.5` |

<a id="database-setup"></a>

## 数据库设置

Multica 要求 PostgreSQL 17。项目不使用 pgvector，没有迁移定义 `vector` 列或执行 `CREATE EXTENSION vector`。附带镜像名为 `pgvector/pgvector:pg17` 仅有历史原因，普通 PostgreSQL 17 即可使用。

迁移涉及四个扩展：

| 扩展 | 迁移 | 是否必需 |
| --- | --- | --- |
| `pgcrypto` | `001_init` | **是**，直接执行 `CREATE EXTENSION`，缺少时迁移失败 |
| `pg_trgm` | `137_search_index_pg_trgm_extension` | **是**，同样直接创建，搜索索引也依赖它 |
| `pg_bigm` | `032_issue_search_index` | 否，通过 `DO ... EXCEPTION` 包裹，不可用时输出 `NOTICE` 并跳过 |
| `pg_cron` | `076_task_usage_pgcron_extension` | 否，同样按条件安装；用量汇总改由进程内执行，见[用量仪表盘汇总](#usage-dashboard-rollup) |

`pgcrypto` 和 `pg_trgm` 随标准 PostgreSQL 17 提供：Debian／Ubuntu 中为 `postgresql-contrib`，Homebrew 的 `postgresql@17` 和官方 `postgres:17` 镜像也包含。因此这两个必需扩展不需要自定义镜像。

`pg_bigm` 可选，仅影响**中日韩文本搜索质量**：二元组索引在中文、日文和韩文上的选择性明显优于三元组索引。没有它仍能搜索，迁移 138–142 会创建可移植的 `pg_trgm` 后备索引，覆盖搜索处理器涉及的列。安装 `pg_bigm` 需要自定义镜像或从源码构建，附带的 `pgvector/pgvector:pg17` 也不包含此扩展。

> `pg_bigm` 和 `pg_cron` 按条件安装，因此 `schema_migrations` 中的记录只能证明迁移执行过，不能证明对象存在。运行 `SELECT extname FROM pg_extension` 检查数据库实际扩展。缺少评论内容搜索索引时，按 `server/cmd/migrate/README.md` 恢复。

<a id="using-docker-compose-recommended"></a>

### 使用 Docker Compose 推荐方式

`docker-compose.selfhost.yml` 已包含 PostgreSQL，无需单独设置。

<a id="using-your-own-postgresql"></a>

### 使用自己的 PostgreSQL

使用已有 PostgreSQL 实例时，请确认迁移账号有权限创建两个必需扩展：

```sql
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

在 `.env` 中设置 `DATABASE_URL`，并从 Compose 文件中移除 `postgres` 服务。

<a id="running-migrations-manually"></a>

### 手动执行迁移

Docker Compose 部署会自动执行迁移。需要手动运行时：

```bash
# 使用已构建的二进制程序
./server/bin/migrate up

# 或从源码运行
cd server && go run ./cmd/migrate up
```

<a id="usage-dashboard-rollup"></a>

## 用量仪表盘汇总

用量和运行时仪表盘读取派生表 `task_usage_hourly`，由 `rollup_task_usage_hourly()` 填充。从 MUL-2957 起，各后端副本通过数据库调度器 `sys_cron_executions` 在**进程内**执行汇总。全新自托管安装无需额外操作，附带的 `pgvector/pgvector:pg17` 镜像可直接使用。

<a id="how-the-in-process-scheduler-works"></a>

### 进程内调度器的工作方式

每个后端副本每 30 秒尝试在 `sys_cron_executions` 中领取当前 5 分钟 UTC 计划。唯一键 `(job_name, scope_kind, scope_id, plan_time)` 保证每个计划只有一个副本领取成功，多实例不会重复写入。处理器随后执行 `SELECT rollup_task_usage_hourly()`，SQL 函数内部持有 advisory lock `4246`，因此遗留的 `pg_cron` 作业或手动调用也能与调度器共存，不会争用汇总操作。可检查审计表确认运行状态：

```sql
SELECT plan_time, status, attempt, runner_id,
       error_code, error_msg, started_at, finished_at
  FROM sys_cron_executions
 WHERE job_name = 'rollup_task_usage_hourly'
 ORDER BY plan_time DESC
 LIMIT 20;
```

<a id="compatibility--existing-pg_cron-registrations"></a>

### 兼容已有 pg_cron 作业

如果此前通过 `SELECT cron.schedule('rollup_task_usage_hourly', '*/5 * * * *', …)` 注册了 `pg_cron` 作业，可以安全保留：advisory lock 4246 会避免重复写入，未获得锁的一方直接结束。进程内调度器正常运行后，可以删除冗余作业：

```sql
SELECT cron.unschedule('rollup_task_usage_hourly')
  FROM cron.job WHERE jobname = 'rollup_task_usage_hourly';
```

直接调用 `SELECT rollup_task_usage_hourly()` 的外部 cron、systemd 或 Kubernetes `CronJob` 仍受支持，它们是 MUL-2957 之前的唯一方式。新部署应使用进程内调度器。

<a id="standalone-backfill-command"></a>

### 独立回填命令

`rollup_task_usage_hourly()` 只处理开始运行之后的新时间桶。如果首次领取汇总计划前，数据库已经存在 `task_usage` 记录，可以用 `backfill_task_usage_hourly` 回填历史时间桶。常见场景是从 `v0.3.4` 升级到 `v0.3.5+`，数据库已有数月用量数据：

```bash
# 在 Docker Compose 中运行
docker compose -f docker-compose.selfhost.yml exec backend \
  ./backfill_task_usage_hourly --sleep-between-slices=2s

# 在 Kubernetes 中运行
kubectl -n multica exec deploy/multica-backend -- \
  ./backfill_task_usage_hourly --sleep-between-slices=2s
```

命令按月分片遍历 `task_usage` 的完整时间范围，调用与进程内调度器相同的幂等操作。可安全重复运行、使用 Ctrl-C 中断，或与调度器并行执行，advisory lock 4246 会将操作串行化。支持以下参数：

| 参数 | 说明 |
| --- | --- |
| `--sleep-between-slices` | 月度分片之间的等待时长，例如 `2s`，用于降低繁忙数据库的读取压力。生产数据库有多年历史时建议设置 |
| `--months-back N` | 仅回填最近 N 个月。**必须同时指定 `--force-partial`**，因为水位线仍会越过更早的时间桶，被跳过的历史桶会被永久放弃 |
| `--dry-run` | 仅记录将处理的分片，不写入数据 |

回填完成后，汇总状态水位线更新为 `now() - 5 minutes`，首次计划执行不会重新处理全部历史。

<a id="v034--v035-upgrade-order"></a>

### v0.3.4 到 v0.3.5+ 的升级顺序

迁移 `103` 增加了保护条件，只有 `task_usage_hourly` 追平后才允许删除旧的每日汇总。从 MUL-2957 起，迁移命令会在应用 `103` 前**自动**在 advisory lock 4246 下执行按月分片的幂等回填，因此 `v0.3.4 → v0.3.5+` 的升级只需执行一次 `migrate up`。

使用 MUL-2957 之前的二进制程序，或因环境问题导致自动钩子失败时，应对同一数据库手动执行 `backfill_task_usage_hourly`，再运行 `migrate up`，也可以重启会自动执行迁移的后端容器。**全新安装不受此限制**：`task_usage` 为空时保护条件直接通过，进程内调度器从首次执行开始处理新时间桶。

<a id="manual-setup-without-docker-compose"></a>

## 手动部署 不使用 Docker Compose

如需自行构建和运行服务：

**前置条件：** Go 1.26.9、Node.js 22、pnpm 10.28.2、PostgreSQL 17。普通 PostgreSQL 安装即可，见[数据库设置](#database-setup)。

```bash
# 启动 PostgreSQL，也可以执行 docker compose up -d postgres

# 构建后端
make build

# 执行数据库迁移
DATABASE_URL="your-database-url" ./server/bin/migrate up

# 启动后端服务
DATABASE_URL="your-database-url" PORT=8080 JWT_SECRET="your-secret" ./server/bin/server
```

前端部署：

```bash
pnpm install
pnpm build

# 以生产模式启动前端
cd apps/web
REMOTE_API_URL=http://localhost:8080 pnpm start
```

<a id="reverse-proxy"></a>

## 反向代理

生产环境应在后端和前端之前配置反向代理，处理 TLS 和路由。

<a id="caddy-recommended"></a>

### Caddy 推荐方式

**单域名部署：** 前后端使用同一主机名，这是 `docker-compose.selfhost.yml` 的默认方式。

```
multica.example.com {
    # WebSocket 路由，必须放在通配路由之前
    @multica_ws path /ws /ws/*
    handle @multica_ws {
        reverse_proxy localhost:8080 {
            flush_interval -1
        }
    }

    # 其他请求转发到前端
    reverse_proxy localhost:3000
}
```

> 单域名部署也需要在后端将 `FRONTEND_ORIGIN`／`CORS_ALLOWED_ORIGINS` 设置为公开来源，例如 `https://multica.example.com`。默认允许列表只包含 localhost，缺少此配置时，公开 URL 的 WebSocket 升级会返回 403，实时更新停止。见[局域网或非 localhost 访问](#lan--non-localhost-access)。

**分域部署：** 前后端使用不同主机名。

```
app.example.com {
    reverse_proxy localhost:3000
}

api.example.com {
    @multica_ws path /ws /ws/*
    handle @multica_ws {
        reverse_proxy localhost:8080 {
            flush_interval -1
        }
    }

    reverse_proxy localhost:8080
}
```

`/ws` 配置中有两处容易忽略，都是 Caddy 部署实时更新异常的常见原因：

- **使用 `path /ws /ws/*`。** 单独的 `handle /ws` 只精确匹配 `/ws`，后续 `/ws/` 下的路径会落到前端。改为 `handle /ws*` 又会匹配过宽：Caddy 的 `*` 没有路径段边界，会连带匹配 `/ws-foo` 等合法工作区路径，只有精确 slug `ws` 被保留。显式列出 `/ws` 和 `/ws/*` 能覆盖所需路径。
- **配置 `flush_interval -1`。** 关闭响应缓冲，让 WebSocket 帧到达后立即转发。否则帧可能等待默认刷新窗口，表现为评论延迟、输入状态缺失或刷新后才能看到评论。

<a id="nginx"></a>

### Nginx

```nginx
# 前端
server {
    listen 443 ssl;
    server_name app.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# 后端 API
server {
    listen 443 ssl;
    server_name api.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 支持 WebSocket
    location /ws {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

前后端使用不同域名时，配置以下环境变量：

```bash
# 后端
FRONTEND_ORIGIN=https://app.example.com
CORS_ALLOWED_ORIGINS=https://app.example.com
COOKIE_DOMAIN=.example.com           # 覆盖两个主机的最窄父域名，详见下方作用域说明

# 前端，仅适用于通过 docker-compose.selfhost.build.yml 从源码构建 Web 镜像
REMOTE_API_URL=https://api.example.com
NEXT_PUBLIC_API_URL=https://api.example.com
NEXT_PUBLIC_WS_URL=wss://api.example.com/ws
```

> **`NEXT_PUBLIC_API_URL` 和 `REMOTE_API_URL` 只填写后端来源，即协议、主机和可选端口，不包含路径。** 使用 `https://api.example.com`，不要写成 `https://api.example.com/api`。浏览器客户端会自行附加前缀，否则请求会变成 `/api/api/...`，头像、图片和附件也会解析到后端未提供的 `<your-path>/uploads/...`，导致页面打开但接口和图片返回 404。末尾的 `/api` 会被防御性移除，其他路径保留，因为反向代理可能确实将后端挂载在某个前缀下。
>
> 升级时尤其需要检查：v0.4.10 之前，发布镜像中的 `NEXT_PUBLIC_API_URL` 不生效，因此错误值可能一直未被发现；v0.4.10 起配置开始生效。升级后界面打开但其他功能异常时，先检查此变量，再重新创建前端容器，例如 `docker compose ... up -d --force-recreate frontend`，以加载新值。

> **此部署方式必须设置 `COOKIE_DOMAIN`。** Web 应用通过 HttpOnly 的 `multica_auth` Cookie 认证，并从 JS 可读取的 `multica_csrf` Cookie 获取值，在所有非 GET 请求中发送 `X-CSRF-Token`。未配置 `COOKIE_DOMAIN` 时，两者都只属于各自主机，`app.example.com` 无法读取 `api.example.com` 签发的 Cookie，也就无法发送 CSRF 请求头。后端会返回 `403 {"error":"CSRF validation failed"}`；GET 仍可读取，表现为页面能打开却无法创建或编辑。
>
> 修改 `COOKIE_DOMAIN` 后，删除**前后端两个主机**现有的 `multica_auth`／`multica_csrf` Cookie，再重新登录，否则旧的主机专属 Cookie 会与新域 Cookie 并存，并被浏览器同时发送。

> **尽量缩小 `COOKIE_DOMAIN` 范围。** 同一设置也控制 `multica_auth` 会话 JWT，浏览器会将它发送给该域名下的每个主机。HttpOnly 只能阻止页面脚本读取，不能阻止其他子域名的服务端收到会话。应选覆盖两个主机的最窄父域名：`agent.example.com` 和 `api.agent.example.com` 应使用 `.agent.example.com`，避免扩大到 `.example.com`，将会话同时发送给独立部署的 `docs.example.com` 等其他主机。

使用此方式必须同时满足以下条件：

- 前后端有共同父域名。`app.com` 和 `api.io` 等无关域名无法由任何 `COOKIE_DOMAIN` 值覆盖。
- 父域名下所有主机均由同一可信主体运营。范围内若有第三方托管服务，或其他团队能够创建记录，被攻陷或恶意的服务就可能收到本部署的有效会话。

不满足任一条件时，使用下方同源部署，让会话 Cookie 仅属于当前主机。

<a id="same-origin-recommended"></a>

### 同源部署 推荐方式

无需单独配置 API 域名。保持 `NEXT_PUBLIC_API_URL` 和 `NEXT_PUBLIC_WS_URL` 为默认值，浏览器通过页面自身来源请求 `/api` 和 `/ws`，从而避免跨主机 Cookie 问题：

```bash
# 后端
FRONTEND_ORIGIN=https://app.example.com
CORS_ALLOWED_ORIGINS=https://app.example.com
COOKIE_DOMAIN=                       # 留空，使 Cookie 仅属于 app.example.com，符合当前部署方式

# 前端
NEXT_PUBLIC_API_URL=                 # 留空，客户端使用相对于页面来源的 /api 路径
NEXT_PUBLIC_WS_URL=                  # 留空
REMOTE_API_URL=http://backend:8080   # Next.js 将 /v1、/api、/auth 和 /uploads 重写代理到此地址
```

通过单一 `app.example.com` 虚拟主机提供所有访问。Next.js 重写会将 `/v1`、`/api`、`/auth` 和 `/uploads` 转发到 `REMOTE_API_URL`，HTTP 可直接使用。WebSocket 不通过这些重写，需要在前端虚拟主机中增加直达后端的 `/ws` 配置：

```nginx
# 添加到上方 app.example.com 的 server 配置块中
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 86400;
}
```

重写无法承载 `Upgrade` 握手的原因，见[局域网或非 localhost 访问中的 WebSocket 配置](#websocket-for-lan--non-localhost-access)。

同源方式将 Cookie、CORS 和 WebSocket 来源检查统一到一个来源，减少配置错误；会话 Cookie 只属于当前主机，其他子域名无法收到。仍可为 CLI 和守护进程保留 `api.example.com` 虚拟主机：这些客户端通过 `Authorization: Bearer` 传入 `mul_` 个人访问令牌，不经过 Cookie 或 CSRF 路径。

<a id="lan--non-localhost-access"></a>

## 局域网或非 localhost 访问

默认通过 localhost 使用 Multica。从局域网其他机器访问，例如 `http://192.168.1.100:3000`，需要让后端接受该来源：

```bash
# .env，将示例地址替换为服务器的局域网 IP
FRONTEND_ORIGIN=http://192.168.1.100:3000
CORS_ALLOWED_ORIGINS=http://192.168.1.100:3000
```

然后重启服务：

```bash
docker compose -f docker-compose.selfhost.yml up -d
```

<a id="websocket-for-lan--non-localhost-access"></a>

### 局域网或非 localhost 的 WebSocket 访问

HTTP 请求，包括插件 API、任务、评论和上传，可以通过 Next.js 将 `/v1`、`/api`、`/auth`、`/uploads` 重写代理到后端。WebSocket 需要的 `Upgrade` 握手不通过这些 HTTP 重写，因此从 `http://<lan-ip>:3000` 访问时，聊天流式响应、任务实时更新和通知等功能需要额外完成以下任一配置：

1. **在服务前配置反向代理，推荐使用。** Nginx 或 Caddy 处理 WebSocket 升级并转发到后端 8080。上方[反向代理](#reverse-proxy)中的 Nginx 示例已包含正确设置 `Upgrade`／`Connection` 请求头的 `location /ws { ... }`。代理配置完成后，浏览器通过代理连接，无需重新构建前端。

2. **将 WebSocket URL 写入 Web 镜像。** 不使用反向代理时，重新构建镜像，将 `NEXT_PUBLIC_WS_URL` 指向后端，且浏览器必须能访问 8080 端口：

   ```bash
   # 在 .env 中配置
   NEXT_PUBLIC_WS_URL=ws://<lan-ip>:8080/ws

   # 重新构建 Web 镜像，将构建期变量写入镜像
   docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build
   ```

   `NEXT_PUBLIC_WS_URL` 是构建期变量，见 `Dockerfile.web`。仅在预构建镜像的 `environment:` 中设置不会生效，必须使用 `selfhost.build.yml` 覆盖文件重新构建镜像。

**还必须将浏览器来源加入允许列表。** 上述方式解决升级代理问题，后端仍会独立检查 WebSocket `Origin`，默认只允许 localhost。通过局域网 IP 或反向代理后的公网域名访问时，在后端将 `CORS_ALLOWED_ORIGINS` 或 `FRONTEND_ORIGIN` 设为准确的浏览器来源并重启，方式与上方[局域网访问配置](#lan--non-localhost-access)相同。否则升级返回 403，后端输出 `websocket: request origin not allowed by Upgrader.CheckOrigin`，浏览器控制台持续输出 `disconnected, reconnecting in 3s`；HTTP 和手动刷新仍可正常使用，因为这些请求与页面同源。一个来源配置同时控制 HTTP CORS 和 WebSocket 来源检查。

> **说明：** 需要因其他原因将不同的公共 API／WebSocket 地址写入 Web 镜像时，也使用相同的源码构建覆盖方式：`docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build`。

<a id="health-check"></a>

## 健康检查

后端提供以下公开健康端点：

```text
GET /health
→ {"status":"ok"}

GET /readyz
→ {"status":"ok","checks":{"db":"ok","migrations":"ok"}}

GET /healthz
→ 与 /readyz 的响应相同
```

`/health` 用于基础存活和可达性检查。`/readyz` 检查数据库和迁移等依赖是否就绪，适合在数据库不可用或迁移未全部完成时应判定失败的探针和外部监控。`/healthz` 作为运维习惯中的别名保留。

<a id="prometheus-metrics"></a>

## Prometheus 指标

后端可以在独立的管理监听端口提供 Prometheus 指标：

```bash
METRICS_ADDR=127.0.0.1:9090 ./server/bin/server
curl http://127.0.0.1:9090/metrics
```

`METRICS_ADDR` 默认留空，不会启动指标监听。公开 API 端口不提供 `/metrics`，面向公网的部署应保持此边界。只有启用指标监听后才会开始累积 HTTP 请求指标。指标可能暴露内部路由、流量、依赖状态和运行时健康信息。

Docker 或 Kubernetes 部署优先使用私有采集路径：将指标监听绑定到内部接口，通过私有网络、白名单、NetworkPolicy 或代理认证保护。在容器内设置 `METRICS_ADDR=0.0.0.0:9090` 时，仅向可信网络发布端口，例如映射为宿主机本地的 `127.0.0.1:9090:9090`。

<a id="go-runtime-profiling"></a>

## Go 运行时性能分析

后端在固定的回环管理地址 `127.0.0.1:6060` 提供全部标准 Go pprof 路由，包括 CPU、heap、allocs、goroutine、block、mutex、threadcreate、symbol 和 trace：

```bash
go tool pprof 'http://127.0.0.1:6060/debug/pprof/profile?seconds=30'
go tool pprof http://127.0.0.1:6060/debug/pprof/heap
```

公开 API 端口不提供 `/debug/pprof/`。管理监听地址不能配置，也不会绑定到容器或宿主机的外部网络接口。采样结果可能暴露进程内部信息，部分采样还会增加 CPU 或内存压力，因此仅应允许同一宿主机或同一容器网络命名空间内的运维人员访问。

容器内回环地址属于该容器的网络命名空间，宿主机无法直接访问。使用 Compose 时，在后端容器内部采集，再复制出来：

```bash
docker compose -f docker-compose.selfhost.yml exec backend \
  wget -qO /tmp/heap.pprof http://127.0.0.1:6060/debug/pprof/heap
docker compose -f docker-compose.selfhost.yml cp backend:/tmp/heap.pprof ./heap.pprof
go tool pprof ./heap.pprof
```

<a id="upgrading"></a>

## 升级

```bash
docker compose -f docker-compose.selfhost.yml pull
docker compose -f docker-compose.selfhost.yml up -d
```

需要固定版本时，在 `.env` 中将 `MULTICA_IMAGE_TAG` 设为 `v0.2.4` 等准确版本。后端启动时自动执行迁移，迁移具有幂等性，重复执行不会重复应用。
如果所选 GHCR 标签尚未发布，改用 `docker compose -f docker-compose.selfhost.yml -f docker-compose.selfhost.build.yml up -d --build`。
