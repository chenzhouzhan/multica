"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import {
  InfrastructureOperationSchema,
  infrastructureResourceKey as resourceKey,
  type InfrastructureConnection,
  type InfrastructureOperation,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../i18n";
import { ErrorMessage, Picker } from "./components";

export function OperationHistory({
  ws,
  connection,
  cluster,
}: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
}) {
  const { t } = useT("runtimes");
  const [scope, setScope] = useState("cluster");
  const operations = useQuery({
    queryKey: [...resourceKey(ws, connection.id), "operations"],
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "list_operations",
        { connection_id: connection.id },
        InfrastructureOperationSchema.array(),
      ),
  });
  return (
    <section className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-subheading">
          {t(($) => $.infrastructure.restart_history)}
        </h2>
        <Picker
          label={t(($) => $.infrastructure.operation_scope)}
          value={scope}
          onChange={setScope}
          items={[
            {
              value: "cluster",
              label: t(($) => $.infrastructure.current_cluster),
            },
            {
              value: "manager",
              label: t(($) => $.infrastructure.current_manager),
            },
          ]}
        />
      </div>
      <ErrorMessage error={operations.error} />
      {operations.data
        ?.filter(
          (operation) =>
            scope === "manager" || operation.cluster_id === cluster,
        )
        .map((operation) => (
          <OperationRow key={operation.id} ws={ws} operation={operation} />
        ))}
      {operations.data?.filter(
        (operation) => scope === "manager" || operation.cluster_id === cluster,
      ).length === 0 && (
        <p className="text-body text-muted-foreground">
          {t(($) => $.infrastructure.no_results)}
        </p>
      )}
    </section>
  );
}
function OperationRow({
  ws,
  operation,
}: {
  ws: string;
  operation: InfrastructureOperation;
}) {
  const { t } = useT("runtimes");
  const final = ["succeeded", "failed", "unknown", "verification_required"];
  const result = useQuery({
    queryKey: [
      ...resourceKey(ws, operation.connection_id),
      "operation",
      operation.id,
    ],
    initialData: operation,
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "get_operation",
        { connection_id: operation.connection_id, operation_id: operation.id },
        InfrastructureOperationSchema,
      ),
    enabled: operation.job_id !== null,
    refetchInterval: (query) =>
      final.includes(query.state.data?.status ?? "unknown") ? false : 5000,
  });
  const row = result.data;
  return (
    <div className="grid gap-2 rounded-lg border p-4 text-body">
      <div className="flex flex-wrap justify-between gap-2">
        <span>{`#${row.cluster_id} / #${row.service_id} / #${row.role_id} · ${row.created_at}`}</span>
        <strong>
          {t(($) => $.infrastructure.operation_status)}: {row.status}
        </strong>
      </div>
      <p className="break-words">{row.reason}</p>
      <p className="text-caption text-muted-foreground">
        {row.actor_type} · {row.actor_id} · {t(($) => $.infrastructure.job_id)}:{" "}
        {row.job_id ?? "—"}
      </p>
      <ErrorMessage error={row.error || result.error} />
      {row.job_id !== null && (
        <Button
          variant="ghost"
          size="sm"
          disabled={result.isFetching}
          onClick={() => result.refetch()}
        >
          {t(($) => $.infrastructure.refresh)}
        </Button>
      )}
    </div>
  );
}
