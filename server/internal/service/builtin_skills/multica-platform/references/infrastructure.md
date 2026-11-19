# Manager resources, service metainfo and role images

Use the daemon-provided `MULTICA_CLI_PATH` executable, or explicitly mounted Multica infrastructure MCP tools. Never call platform or Manager HTTP endpoints yourself.

Discover the Manager connection UUID, then cluster and service IDs; numeric IDs can repeat across Managers.

```bash
"$MULTICA_CLI_PATH" infrastructure managers
"$MULTICA_CLI_PATH" infrastructure clusters --manager <connection-uuid>
"$MULTICA_CLI_PATH" infrastructure services --manager <connection-uuid> --cluster <cluster-id>
"$MULTICA_CLI_PATH" infrastructure metainfo list --manager <connection-uuid> --service <service-id>
"$MULTICA_CLI_PATH" infrastructure metainfo read --manager <connection-uuid> --service <service-id> --key '<listed-key>' --output text > metainfo.yaml
"$MULTICA_CLI_PATH" infrastructure metainfo save --manager <connection-uuid> --service <service-id> --key '<listed-key>' --file metainfo.yaml --reason '<reason>'
```

MCP equivalents are `infra_list_metainfo_files`, `infra_read_metainfo_file`, and `infra_save_metainfo_file`. Each requires `connection_id` and `service_id`; read/save also require the returned `key`, and save requires `content` and `reason`.

Read before writing. Metadata belongs to a service type/version within one Manager and is shared by its instances across clusters. Save only a change the user requested. Writes require the task's human originator to still be a workspace owner/admin; runtime ownership grants no extra authority. An unattributed task cannot save.

CLI output defaults to JSON. `read --output text` preserves the raw content. `save --file -` reads stdin; the CLI accepts UTF-8 content up to 1 MiB. A successful save returns `saved: true` and an `audit_id`. Read again to verify. An uncertain submission must be inspected before retrying. `infra_get_operation` polls role restarts, not metainfo audit IDs.

## Service images and Pod restarts

Use the existing service-scoped publication pipeline. Do not SSH to nodes, request node credentials, retag a shared Manager image, or run kubectl yourself. An administrator must have configured the cluster's execution access in the application. If discovery returns `configured: false`, explain that prerequisite.

Discover actual target keys and current images first:

```bash
"$MULTICA_CLI_PATH" infrastructure images list --manager <connection-uuid> --cluster <cluster-id> --service <service-id>
```

Create a JSON array containing 1–16 selected targets. Use exact returned keys and the user's complete Harbor source references. Different roles can use different images and arbitrary valid tags:

```json
[
  {"key":"deployment/logs/worker-a/worker","source":"harbor.example/team/worker:custom_fix_42"},
  {"key":"deployment/logs/coordinator-a/coordinator","source":"harbor.example/team/coordinator:other_fix"}
]
```

```bash
"$MULTICA_CLI_PATH" infrastructure images preview --manager <connection-uuid> --cluster <cluster-id> --service <service-id> --file images.json --reason '<requested change>'
"$MULTICA_CLI_PATH" infrastructure images get --manager <connection-uuid> --operation <local-operation-uuid>
"$MULTICA_CLI_PATH" infrastructure images start --manager <connection-uuid> --cluster <cluster-id> --service <service-id> --operation <local-operation-uuid>
"$MULTICA_CLI_PATH" infrastructure images get --manager <connection-uuid> --operation <local-operation-uuid>
```

For a requested Pod restart, use `preview --action restart_pods`; each selection needs only `key`. It fixes the current image version and restarts the selected workload's Pods, so other containers in that Pod also restart. It does not restart other service workloads. `--file -` reads the JSON selections from stdin.

MCP equivalents: `infra_list_service_images`, `infra_preview_service_images`, `infra_start_service_images`, then `infra_get_operation`. The first three require `connection_id`, `cluster_id`, and `service_id`. Preview also takes `action` (`publish_images` or `restart_pods`), structured `items`, and `reason`; start takes the returned `operation_id`. Polling requires `connection_id` and `operation_id`.

Preview and start require the task's human originator to still be a workspace owner/admin. Preview is bound to the Agent, task and originator; another task cannot submit it. Audit records attribute the Agent, task, originator, reason and per-role outcome. Node credentials stay on the backend. Administrator SSH settings remain a human-only operation.

When the user has requested the image change or restart, carry discovery, preview, execution and verification through without adding another approval step. A request only to preview stops at `ready`. Poll while `preparing`; at `ready`, inspect the selected targets and pinned images, then start the same preview. It expires after 10 minutes. Poll `running` until the rollout and actual Pod images are verified. Report completion only at `succeeded`.

For `unknown`, inspect the operation and current deployments; never automatically resubmit an uncertain write. `verification_required` means a deployment was updated but verification is incomplete; polling performs read-only checks. `failed`, `conflict` and `expired` need their recorded details examined before any new preview. Partial results remain visible and completed roles must not be reported as failed or silently retried.
