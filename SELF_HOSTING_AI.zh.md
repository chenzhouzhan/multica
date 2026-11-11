<a id="self-hosting-setup-for-ai-agents"></a>

# 自托管部署步骤 供 AI 智能体执行

[English](SELF_HOSTING_AI.md) | 简体中文

本文供 AI 智能体执行。请严格按以下步骤部署本地 Multica 实例并连接。

<a id="prerequisites"></a>

## 前置条件

- 已安装 Docker 和 Docker Compose。
- 已安装 Homebrew，用于安装 CLI。
- PATH 中至少有一个 AI 智能体 CLI：`claude` 或 `codex`。

<a id="install"></a>

## 安装

```bash
# 安装 CLI 并部署自托管服务端
curl -fsSL https://raw.githubusercontent.com/multica-ai/multica/main/scripts/install.sh | bash -s -- --with-server

# 配置 CLI 连接 localhost，完成认证并启动守护进程
multica setup self-host
```

等待服务端输出 `✓ Multica server is running and CLI is ready!` 后，再运行 `multica setup self-host`。

**预期结果：**

- 前端地址为 http://localhost:3000。
- 后端地址为 http://localhost:8080。
- `multica` CLI 已安装，并配置为连接 localhost。

<a id="alternative-manual-setup"></a>

## 备选方案 手动部署

```bash
git clone https://github.com/multica-ai/multica.git
cd multica
make selfhost
brew install multica-ai/tap/multica
multica setup self-host
```

`multica setup self-host` 命令会：

1. 配置 CLI 连接 localhost:8080／localhost:3000。
2. 打开浏览器登录，使用邮件中的验证码；未配置 Resend 时，使用后端日志输出的验证码。
3. 自动发现工作区。
4. 在后台启动守护进程。

<a id="verification"></a>

## 验证

```bash
multica daemon status
```

输出应显示 `running`，并列出检测到的智能体。

<a id="stopping"></a>

## 停止

```bash
# 停止守护进程
multica daemon stop

# 停止所有 Docker 服务
cd multica
make selfhost-stop
```

<a id="custom-ports"></a>

## 自定义端口

如果默认的 8080／3000 端口已被占用：

1. 编辑 `.env`，修改 `PORT` 和 `FRONTEND_PORT`。它们是宿主机端口，容器内部仍监听 8080／3000，因此无需重新构建。
2. 运行 `make selfhost`。
3. 运行 `multica setup self-host --port <PORT> --frontend-port <FRONTEND_PORT>`。

请直接修改文件：`make` 会通过 `include` 读取 `.env`，文件中的值优先于 Shell 环境中的同名变量。例如，`.env` 设置了 `PORT` 时，`PORT=9100 make selfhost` 中的环境变量会被覆盖。命令行赋值 `make selfhost PORT=9100` 则会生效。直接运行 Docker Compose 时，优先级相反，环境变量优先于 `.env`。

`BACKEND_PORT` 是覆盖后端 `PORT` 的可选别名，之后依次为 `API_PORT`、`SERVER_PORT`。无论通过哪种方式启动，脚本都会从 Docker Compose 回读实际发布的端口，因此输出地址与服务真正使用的地址一致。

<a id="troubleshooting"></a>

## 故障排查

- **后端未就绪：** `docker compose -f docker-compose.selfhost.yml logs backend`
- **前端未就绪：** `docker compose -f docker-compose.selfhost.yml logs frontend`
- **守护进程异常：** `multica daemon logs`
- **健康检查：** `curl http://localhost:8080/health` 检查存活状态，`curl http://localhost:8080/readyz` 检查包含依赖状态的就绪情况。
