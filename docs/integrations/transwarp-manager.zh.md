# 星环 Manager 服务器资源管理

入口：工作区 → 集群。Web 和桌面端共用同一页面。

二级导航顶部切换 Manager，下方搜索并选择该 Manager 的集群；连接的增删改集中在底部“管理 Manager”入口。主区默认按产品分区、版本分层展示服务，产品和版本均可独立收起；产品筛选包含该产品的全部版本。可按名称、类型或版本搜索，并筛选非健康服务；搜索会展开匹配的分区。进入服务后使用整页详情，通过“角色 / 日志 / metainfo”切换内容；返回列表保留筛选。节点和操作记录是集群级页签，角色重启记录可切换当前集群或当前 Manager 范围。

依赖关系通过服务详情标题旁的入口按需打开，展示依赖方和被依赖方，包含全局服务及跨集群服务；箭头指向被依赖的服务。

一个工作区可以连接多个 Manager；每个 Manager 下分别选择集群、服务及角色。资源的数字 ID 只在所属 Manager 内有意义，给智能体下任务时请同时指定 Manager 名称和集群。

## 部署与连接

后端配置 `MULTICA_INFRASTRUCTURE_SECRET_KEY`，值为 `openssl rand -base64 32` 生成的密钥。它用于加密数据库中的 Manager 密码；请持久保存并备份，丢失后需要重新配置连接凭据。数据库需要应用 615 迁移，智能体所在机器的 Multica daemon 也需要更新到包含此功能的版本。

工作区 owner/admin 可添加 Manager，填写名称、HTTPS 地址、账号、密码，以及内部证书所需的 CA PEM。每个连接独立保存凭据和登录会话。若不校验证书，可勾选“忽略证书验证”；默认关闭，开启后仅跳过该 Manager 的证书信任和主机名校验，CA 输入框同时停用。保存前会登录并读取集群列表；编辑时密码留空则保留原密码。修改连接会清除其重启授权，需重新选择允许的角色。

客户端使用文档中的 `POST /api/users/login` 并读取当前用户资料确认会话。Manager 若要求验证码或二次验证，登录可能被拒绝；当前版本不支持交互式验证码登录。它不会关闭或绕过 Manager 的验证要求。尚需使用实际环境的专用账号验证该登录方式。

## 查看资源

选择 Manager 和集群后可查看节点、服务和服务角色；日志查询按服务、主机和级别执行，默认最近 15 分钟，单次最多 24 小时。资源响应只保留页面和工具所需字段，不透传节点仓库中的代理密码等配置。

## 服务 Metainfo

服务详情标题旁点击“Metainfo”，默认打开该服务版本的主 YAML 文件，文件按目录树展示，可展开和收起目录，切换多语言文件及其他资源文本。编辑器使用 CodeMirror，支持 YAML、properties 语法高亮、行号、缩进、搜索和撤销。工作区成员可查看，owner/admin 可进入编辑并保存到 Manager；二进制文件仅标记为不可文本编辑。保存失败时保留输入，关闭未保存的编辑会提示放弃修改。

metainfo 使用目录树和 YAML / properties 编辑器；切换文件或通过应用链接离开时提示未保存修改，草稿按工作区、Manager、服务类型、版本和文件隔离保存，刷新后可恢复。

文件范围是“Manager + 服务类型 + 服务版本”，与当前集群无关；修改会影响该 Manager 内所有同类型、同版本的服务实例。此入口只操作 Manager 返回的 SERVICE 文件树，智能体可通过正式 MCP 或 CLI 读取和编辑；保存要求任务发起人当前仍为工作区 owner/admin，不能借用运行机器所有者的权限。每次保存记录发起请求和保存结果，包含 Agent、任务、Manager、文件路径、修改原因及前后内容哈希，不记录文件正文。

## 自动重启授权

管理员在角色列表点击“已授权 / 未授权”打开授权设置，逐个勾选“允许自动重启”。授权绑定当前 Manager、集群、服务和角色，默认不允许重启。成员和智能体均需已有授权并填写原因。

执行时重新检查角色归属、安装状态及 Manager 的可停止检查，然后写入本地操作记录，再提交一次角色重启。返回 Manager 任务 ID 只表示已提交。页面和智能体查询操作结果时，会读取任务状态，并在成功后检查角色是否运行且健康。

`unknown` 表示提交结果不确定，`verification_required` 表示任务完成但角色尚未验证为健康。这两种情况请先到对应 Manager 核实，不要直接重复提交。客户端不会自动重试重启请求。

## 智能体工具

有连接且配置了加密密钥的工作区，在新任务启动时向新版 daemon 提供以下原生 MCP 工具。凭据留在后端，daemon 只转发当前运行任务的调用。

- `infra_list_managers`、`infra_list_clusters`
- `infra_list_nodes`、`infra_list_services`、`infra_list_roles`、`infra_service_relations`
- `infra_list_metainfo_files`、`infra_read_metainfo_file`、`infra_save_metainfo_file`
- `infra_search_logs`、`infra_node_metrics`
- `infra_restart_role`、`infra_list_operations`、`infra_get_operation`

示例任务：

> 检查 Manager“生产区 A”下“日志集群”的服务健康状态，读取异常角色最近 15 分钟的 ERROR 日志。仅对已授权角色执行必要的重启，记录原因，查询任务完成状态并验证健康；提交结果不确定时停止重复操作，在任务中报告。

读取资源需要 Manager 账号具备对应权限，重启还需要该账号拥有 Manager 中的操作权限。移除连接会删除其本地操作记录和授权，不会卸载或停止上游服务。

## CLI 读取与编辑 Metainfo

升级后端和 daemon 后，新任务可发现上述 MCP 工具。运行中的旧任务可重新检查其 `MULTICA_CLI_PATH` 对应 CLI 的帮助；MCP 工具目录在新任务启动时挂载。

```bash
"$MULTICA_CLI_PATH" infrastructure managers
"$MULTICA_CLI_PATH" infrastructure clusters --manager <connection-uuid>
"$MULTICA_CLI_PATH" infrastructure services --manager <connection-uuid> --cluster <cluster-id>
"$MULTICA_CLI_PATH" infrastructure metainfo list --manager <connection-uuid> --service <service-id>
"$MULTICA_CLI_PATH" infrastructure metainfo read --manager <connection-uuid> --service <service-id> --key '<listed-file-key>' --output text > metainfo.yaml
"$MULTICA_CLI_PATH" infrastructure metainfo save --manager <connection-uuid> --service <service-id> --key '<listed-file-key>' --file metainfo.yaml --reason '说明本次修改'
```

CLI 默认输出 JSON，`read --output text` 原样输出文件内容。保存支持 `--file -` 从 stdin 读取，单个内容文件最多 1 MiB。保存成功返回 `saved: true` 和 `audit_id`；保存后再次读取核对。提交结果不确定时先读取核实，不自动重试保存，也不使用 curl/wget 绕过正式工具链。
