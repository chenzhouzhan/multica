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

## 角色镜像发布与 Pod 重启

入口：集群 → 服务详情 →“镜像发布”；角色行的“更新镜像”也会进入当前服务的发布页。每个实际工作负载、容器分别显示当前镜像，按项勾选并填写公司 Harbor 的完整源镜像地址；不同角色可以使用不同仓库和任意合法 tag。未选容器及同集群的其他服务不会被发布操作修改。

管理员首次在当前集群点击“配置执行节点”，保存 Manager 节点的 SSH 主机、端口、用户、认证方式和登录凭据，无需手填主机公钥。首次连接自动读取服务器的 SHA256 指纹，核对后点击“信任并继续”才会发送登录凭据；后续连接校验已保存的主机公钥。主机身份变化时会拒绝登录，可通过“重新读取指纹”显式重新核对。远程命令使用该 SSH 用户的 Bash 交互式登录环境，支持配置文件中的 `kubectl` 别名、函数和环境变量，包括别名中指定的 API 地址与证书参数。集群连接由系统通过该用户实际使用的 `kubectl` 自动读取；只有一套时直接使用，即使未设置 current-context 也可识别。发现多套时，页面显示“集群连接”选择项，选中后再保存；不会修改节点的默认 context。支持 SSH 密码或无口令私钥；凭据使用 `MULTICA_INFRASTRUCTURE_SECRET_KEY` 加密，读取接口只返回公开连接信息。相同 SSH 主机和用户下，密钥或密码留空则保留原值；更换身份时需重新提供凭据。保存会校验 SSH、Docker 客户端和 Kubernetes 连通性；Kubernetes 校验失败时会显示 SSH 用户及远程命令的错误。修改 Manager 地址后需要重新配置执行节点。

后端须能通过 SSH 访问执行节点；该用户须已配置 Docker 仓库登录、kubeconfig，以及读取工作负载、ReplicaSet、Pod 和修改目标工作负载的权限。集群节点须能直接访问公司 Harbor，并已配置相应 `imagePullSecrets`；SSH 节点上的 `docker login` 不会替 Pod 配置拉取凭据。本流程直接使用 Harbor 镜像，不覆盖 Manager 仓库中的共享 tag。数据库需要应用 620 迁移。

资源范围来自 Manager 的服务 Kubernetes 对象列表及其实际清单，再从执行节点读取当前工作负载 UID、generation、容器和镜像。仅在容器名与 Manager 角色类型精确对应时自动关联角色名称；无法对应时显示实际容器、namespace 和工作负载，供管理员明确选择，不按名称猜测跨服务归属。

“预览所选角色”创建后台操作，读取最新部署，并在执行节点解析各镜像的不可变 digest；普通 tag 会先 `docker pull`，随后读取指定 digest 的 manifest，兼容 Docker v2 和 OCI 镜像。Docker 17 等不支持 `manifest inspect --verbose` 的客户端会按固定 digest 拉取，再用 `docker image inspect` 核对仓库 digest 和本地镜像 ID；该方式只记录执行节点平台的镜像 ID，其他平台的 Pod 若返回不同的内容摘要会保留为待校验。预览不修改 Kubernetes，页面展示各容器的当前镜像、源镜像与固定引用。准备操作最多运行 30 分钟，完成后的预览保留 10 分钟。确认发布只能由创建预览的管理员提交一次；服务、角色、镜像或执行配置变化时需重新预览。

执行前重新核对服务归属与所选容器；Kubernetes JSON Patch 使用 UID、generation、容器名和旧镜像的原子检查，只替换选中的容器镜像。同一工作负载的多个容器合并为一次修改；不同工作负载依次执行。随后等待 rollout，并按控制器归属检查 Pod Ready、容器镜像及运行时内容摘要，不会仅凭命令退出成功报告发布成功。暂停、缩容为零及 `OnDelete` 更新策略的工作负载不支持此发布流程。

“重启角色 Pod”同样只作用于勾选的容器所处工作负载：先解析并预览当前镜像引用对应的 digest，确认后固定该版本并更新 Pod 模板的重启注解。Kubernetes 会重建该工作负载的 Pod，因此同一工作负载中的其他容器也会随 Pod 重建，但其镜像字段保持不变。

操作记录保留每个容器的原镜像、固定镜像、步骤和结果。某一步失败后停止后续工作负载，保留已完成的结果，不自动回退或重复提交。`unknown` 表示写入结果不确定，先核实当前部署；`verification_required` 表示部署已更新但 Pod 或实际镜像尚未验证，处理后可点击“刷新”重新进行只读校验，不会再次写入。批量操作中未执行的角色需另建预览。服务详情链接携带操作 ID，刷新页面后可继续查看结果；集群“操作记录”也可返回该服务的发布页。

Agent 可通过正式 MCP 工具 `infra_list_service_images`、`infra_preview_service_images`、`infra_start_service_images` 和 `infra_get_operation` 完成同一流程；CLI 对应 `infrastructure images list / preview / start / get`。预览和提交均校验任务仍在运行，且任务发起人当前仍为工作区 owner/admin；预览绑定 Agent、任务和发起人，其他任务不能接管提交。操作记录保存 Agent、任务、发起人及各角色结果，SSH 凭据不下发给 Agent。用户已明确要求更换镜像或重启时，Agent 可完成预览、提交和验证，无需增加审批；只要求预览时停在 `ready`。执行节点 SSH 设置仍由管理员在页面配置。

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
