# Manager resources and service metainfo

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
