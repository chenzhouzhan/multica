# Multica 插件开发指南

Multica 插件可以在 Web 和桌面端增加界面、调用外部服务、向智能体提供工具，并附带 skill。本文按当前仓库的实现说明安装与开发流程。

## 选择扩展方式

| 方式 | 调用方向 | 适合的场景 |
| --- | --- | --- |
| Action | 插件调用 Multica | 读取任务、修改标题或描述、发表评论、保存插件数据 |
| Hook | Multica 调用插件 | 调用业务服务、响应任务事件、定时同步、提供智能体工具 |
| Resource | 安装静态资源，无运行时调用 | 将 `SKILL.md` 安装为工作区 skill |

界面扩展称为 surface，运行在沙箱 iframe 中。清单支持 `issue_panel`（任务面板）、`sidebar_panel`（侧边栏面板）、`modal`（弹窗），平台可声明为 `web`、`desktop`。

Multica 托管插件上传的前端文件。Hook 处理服务与 MCP 服务由插件作者自行部署，Multica 不执行压缩包里的服务端程序。

## 安装前提

工作区的 `owner` 或 `admin` 可以发布、安装和管理插件。插件功能受 `plugins_v1` 开关控制，未配置时默认关闭。自托管部署可向 **Multica 后端进程**传入：

```dotenv
FF_PLUGINS_V1=on
```

带界面的插件还需要以下部署配置：

| 变量 | 用途 |
| --- | --- |
| `MULTICA_PLUGIN_SECRET_KEY` | Base64 编码的 32 字节密钥，用于加密配置中的 secret 和界面启动 URL |
| `MULTICA_PLUGIN_SURFACE_ORIGIN` | 独立、无 Cookie 的插件内容 origin，例如 `https://plugin-content.example.com` |
| `MULTICA_PLUGIN_API_URL` | 外部 Hook 回调的完整 API 基地址，例如 `https://plugin-api.example.com/v1`；未设置时使用 `MULTICA_PUBLIC_URL` 加 `/v1` |

可用 `openssl rand -base64 32` 生成部署密钥，通过部署环境保存。插件内容 origin 必须与应用及 API origin 不同，反向代理需将请求路由到 Multica 后端并保留 `Host`，应用 Cookie 不应覆盖该域名。

向部署配置加入变量后，按现有部署流程重启服务。仓库开发环境使用 `make up`、`make status`、`make down` 管理；完整环境变量说明见[中文环境变量文档](../../apps/docs/content/docs/environment-variables.zh.mdx)。

## 先运行 Hello Panel

[Hello Panel](hello-panel/) 不需要额外后端或构建步骤，适合验证插件安装和界面桥接。

从仓库根目录进入示例目录，再打包清单和入口脚本：

```bash
cd examples/plugins/hello-panel
zip -r /tmp/multica-hello-panel.zip multica.plugin.json ui
```

ZIP 根目录应直接包含 `multica.plugin.json`，不要再包一层 `hello-panel/`。

1. 打开目标工作区的 **设置 → 插件**，上传 ZIP，发布一个版本。
2. 选择该版本，查看权限并确认安装。
3. 打开一个任务，查看 Hello 面板。
4. 保存个人备注，再刷新页面，确认备注仍然存在。
5. 点击面板中的测试评论按钮，确认评论以当前用户身份发布，并带有插件来源记录。

发布范围是当前工作区。**发布与安装是两个步骤**：上传新版本不会自动替换已安装版本，需要管理员选择版本并确认升级。上传时重复使用已经发布的版本号会被拒绝，应修改清单里的 `version`。

## 插件目录与清单

最小目录结构：

```text
hello-panel/
├── multica.plugin.json
└── ui/
    └── main.js
```

以下是 Hello Panel 的清单结构，展示名称可自行调整：

```json
{
  "manifest_version": 1,
  "key": "ai.multica.hello-panel",
  "name": "Hello Panel",
  "version": "1.0.0",
  "author": { "name": "multica", "url": "https://multica.ai" },
  "scopes": ["issues:read", "comments:write", "storage:user"],
  "contributes": {
    "surfaces": [
      {
        "key": "hello",
        "type": "issue_panel",
        "name": "Hello",
        "entry": "ui/main.js",
        "platforms": ["web", "desktop"]
      }
    ]
  }
}
```

`key` 使用反向域名形式，开发自己的插件时换成自己的命名空间。`version` 使用语义化版本号。`entry` 是相对包根目录的文件路径，上传时必须包含对应文件。`scopes` 声明插件需要的权限，管理员在安装时授权。

## 使用前端 SDK

SDK 源码位于 [`packages/plugin-sdk`](../../packages/plugin-sdk/)，包名为 `@multica/plugin-sdk`。以下是需要打包的源代码示例，展示读取任务、添加按钮和保存个人备注：

```js
import { multica } from "@multica/plugin-sdk";

async function main() {
  const root = document.getElementById("root");
  const issue = await multica.issue.get();
  const title = document.createElement("p");
  title.textContent = issue.title;

  const button = document.createElement("button");
  button.textContent = "记录检查结果";
  const status = document.createElement("p");

  button.onclick = async () => {
    button.disabled = true;
    try {
      await multica.issue.comment({ body: "已通过插件完成检查。" });
      await multica.storage.user.set("last-checked-issue", issue.id);
      status.textContent = "已记录";
    } catch (error) {
      status.textContent = error.message;
    } finally {
      button.disabled = false;
      multica.ui.resize(document.body.scrollHeight + 16);
    }
  };

  root.append(title, button, status);
  multica.ui.resize(document.body.scrollHeight + 16);
}

main().catch((error) => {
  document.getElementById("root").textContent = error.message;
  multica.ui.resize(document.body.scrollHeight + 16);
});
```

将 SDK 和自己的依赖打包为一个 `ui/main.js` 后再上传。宿主生成包含 `#root` 的页面；入口需要是单文件脚本，不能保留需要浏览器解析的顶层模块导入，也不能依赖额外的模块文件或动态分块。Hello Panel 示例直接内置最小桥接客户端，因此无需构建；使用 SDK 时应走打包流程。

常用接口与对应权限：

| 接口 | 用途 | 权限 |
| --- | --- | --- |
| `multica.context.get()` | 当前工作区、用户、任务及非 secret 配置 | 由宿主提供上下文 |
| `multica.issue.get(issueId?)` | 读取任务 | `issues:read` |
| `multica.issue.update(patch, issueId?)` | 修改任务标题、描述 | `issues:write` |
| `multica.issue.comments(issueId?)` | 读取评论 | `comments:read` |
| `multica.issue.comment({ body, parentId? }, issueId?)` | 发布评论或回复 | `comments:write` |
| `multica.storage.user` | 当前成员的插件数据 | `storage:user` |
| `multica.storage.workspace` | 工作区共享的插件数据 | `storage:workspace` |
| `multica.hooks.invoke(hookKey, input?)` | 调用本插件声明了 `ui` 触发器的 Hook | 需满足 Hook 的授权与网络限制 |
| `multica.ui.resize(height)` | 申请 iframe 高度，单位为 CSS 像素 | 宿主限制最终高度 |
| `multica.ui.onThemeChange(fn)` | 监听主题变化，返回取消订阅函数 | 无额外数据权限 |

`issueId` 省略时使用当前面板所属任务；没有任务上下文时需要显式传入。`update` 当前只接受 `title`、`description`，不能把它当作通用任务编辑 API。

存储接口提供 `list()`、`get(key)`、`set(key, value)`、`delete(key)`。值为字符串，保存对象时自行用 `JSON.stringify` 编码；读取不存在的键返回 `null`。

## 界面的权限与运行环境

界面通过 SDK 向宿主发消息，宿主使用当前登录用户的会话执行操作。插件同时受**管理员授予的 scopes**和**当前用户自身权限**限制。未授权的 scope 会产生带状态码的 `MulticaPluginError`。

面板发表评论时，作者是当前用户，并记录插件来源；通过这个入口发出的 `@mention` 不会触发智能体运行。

沙箱使用 `sandbox="allow-scripts"`，不开放 `allow-same-origin`：

- 使用 `multica.storage` 保存状态，不能依赖 `localStorage`、`sessionStorage` 或 Cookie。
- 通过宿主提供的 CSS 变量适配主题，例如 `var(--background)`、`var(--foreground)`、`var(--border)`。
- 内容变化后调用 `multica.ui.resize()`；iframe 不会自动适配高度。
- 网络访问需要声明并获得相应的 `net:<域名>` 权限。未声明时，界面的 `connect-src` 为 `'none'`。

`net:example.com` 不包含 `api.example.com`，需要分别声明。界面直接请求自己的服务时，浏览器请求的 `Origin` 为 `null`，服务端需按实际需求处理 CORS。调用已声明的 Hook 时优先使用 `multica.hooks.invoke()`，由宿主完成签名与回调授权。

## Hook：连接自己的业务服务

Hook 在 `contributes.hooks` 中声明。一个 Hook 可以有多个触发器：

| 触发器 | 调用入口 |
| --- | --- |
| `ui` | 插件界面的 `multica.hooks.invoke()` |
| `manual` | 用户在任务操作菜单中调用 |
| `agent` | 智能体将它作为工具调用 |
| `event` | 订阅产品事件，异步执行 |
| `schedule` | 按清单中的 cron 定时执行 |

`transport.type` 支持 `http` 和 `mcp`，地址必须为 HTTPS，目标主机需要被 `net:` scope 覆盖。`event`、`schedule` 不阻塞用户操作。

事件订阅支持任务创建、更新、状态变化、评论创建及运行开始、完成、失败，具体名称见[清单契约](../../server/pkg/plugincontract/manifest.go)。订阅时需声明对应读取权限，例如 `issue.created` 需要 `issues:read`。

HTTP Hook 的完整清单和处理程序见 [Triage Notify](triage-notify/)。处理程序应对原始请求字节验签、校验时间戳、使用常量时间比较，并在有效窗口内记录已处理签名以避免重复处理。

每次调用带有短期 `callback_token` 和 `callback_url`。回调凭据在该次 HTTP 请求结束时撤销，必须在响应前完成相关回调。需要在响应后继续工作的服务，应使用管理员生成的长期安装令牌 `mpi_...`，通过 Plugin Public API 访问已授权能力。

管理员生成或轮换令牌时，安装令牌和 `whsec_...` 签名密钥只展示一次。签名密钥放到插件后端，不能打进界面脚本。`ui`、`manual` 回调以发起用户身份写入；`event`、`schedule` 回调以插件身份写入。

### 定时 Hook

`schedule` 触发器需要 `schedule.cron` 和 `schedule.timezone`，当前仅支持 HTTP 传输。cron 使用五段格式，执行间隔不得短于五分钟。例如每五分钟执行一次：

```json
{
  "triggers": ["schedule"],
  "schedule": {
    "cron": "*/5 * * * *",
    "timezone": "UTC"
  }
}
```

这是 Hook 中的字段片段，完整清单见 [Schedule Pulse](schedule-pulse/multica.plugin.json)。定时投递可能重试，同一个计划执行时间会复用 `delivery_id`；处理程序应在执行副作用前持久化并检查该 ID。不要用每次尝试都会变化的 `invocation_id` 去重。

### MCP 工具

插件可通过 `mcp` 传输接入外部 MCP 服务。安装后管理员需要在工具审批面板明确批准工具，未批准的工具不可调用。工具的输入 schema 发生变化后，原有批准不能直接覆盖新 schema。

完整示例见 [Deploy Sentinel](deploy-sentinel/)，包含 HTTP Hook、MCP 服务、界面和配套 skill。

## 附带 skill 与配置表单

skill 在 `contributes.resources` 中声明，文件路径遵循 `skills/<key>/SKILL.md`：

```json
{
  "type": "skill",
  "key": "incident-response",
  "entry": "skills/incident-response/SKILL.md"
}
```

这是 resources 数组中的一个条目。安装时它成为工作区 skill，卸载插件时会移除。可参考 [Deploy Sentinel 的 skill](deploy-sentinel/skills/incident-response/SKILL.md)，将工具使用顺序和业务约束写在一起。

清单顶层 `config` 可声明 `string`、`number`、`bool`、`enum`、`secret` 字段，宿主生成配置表单。普通配置通过 `(await multica.context.get()).config` 获取；`secret` 不会传到 iframe。

## 本地开发

后端设置 `MULTICA_PLUGIN_DIR` 后，可以从该目录下的插件文件夹发布，省去反复压缩上传。例如将它设为仓库 `examples/plugins` 的绝对路径，然后在插件设置中选择本地目录名 `hello-panel` 发布。

本地发布仍会创建固定版本；重复发布相同版本号时生成 `+dev.N` 版本。**它不是热重载**，发布后仍需安装或升级到对应版本。

需要请求本地 Hook 服务时，还要配置：

| 变量 | 用途 |
| --- | --- |
| `MULTICA_PLUGIN_DEV_ORIGINS` | 允许开发时访问的本地 HTTPS origin，多个值用逗号分隔，例如 `https://localhost:8787` |
| `MULTICA_PLUGIN_DEV_CA` | 用于信任上述开发服务证书的 PEM CA 文件路径 |

这些配置不会放宽清单中的 `net:` 授权，也不会关闭 TLS 校验。清单、服务证书、域名解析和 `net:` 域名需要一致。具体本地服务启动方式见 [Deploy Sentinel 的运行说明](deploy-sentinel/README.md#running-it-locally)。

## 常见问题

| 现象 | 检查项 |
| --- | --- |
| 看不到插件入口或提示管理功能未启用 | 后端 `plugins_v1` 是否启用；管理操作是否使用 `owner`、`admin` 账号 |
| 上传后界面没有变化 | 是否只发布了版本，尚未安装或升级 |
| 面板无法启动 | 内容 origin、密钥、反向代理 `Host` 配置；入口是否打成单文件 |
| 读取或写入返回 403 | 管理员授予的 scopes、当前用户权限和安装启用状态 |
| 界面网络请求被拦截 | `net:` 是否精确覆盖目标域名，目标服务是否正确处理 CORS |
| 本地 Hook 无法连接 | HTTPS 证书、开发 origin、CA、域名解析及网络可达性 |
| MCP 工具不可调用 | 工具是否已审批，schema 是否在审批后发生变化 |
| 定时业务重复执行 | 是否按稳定的 `delivery_id` 持久化去重 |

## 源码与示例索引

- [Hello Panel](hello-panel/)：最小任务面板，无需额外服务。
- [Release Checklist](release-checklist/)：另一份界面插件示例。
- [Triage Notify](triage-notify/)：HTTP Hook、事件与回调。
- [Schedule Pulse](schedule-pulse/)：定时触发、存储与重复投递去重。
- [Deploy Sentinel](deploy-sentinel/)：组合 HTTP、MCP、skill 与界面的业务示例。
- [前端 SDK](../../packages/plugin-sdk/index.ts)：可调用方法与参数。
- [清单解析与校验](../../server/pkg/plugincontract/manifest.go)：清单格式、权限和限制。
- [插件管理界面](../../packages/views/settings/components/plugins-tab.tsx)：发布、安装、配置和工具审批入口。
