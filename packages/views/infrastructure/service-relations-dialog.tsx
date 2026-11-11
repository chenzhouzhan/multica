"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  InfrastructureServiceRelationsSchema,
  infrastructureResourceKey as resourceKey,
  clusterHref,
  type InfrastructureConnection,
  type InfrastructureService,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { ArrowRight, GitBranch } from "@multica/ui/icons";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { ErrorMessage } from "./components";

export function ServiceRelationsDialog(props: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
  service: InfrastructureService;
  services: InfrastructureService[];
}) {
  const { t } = useT("runtimes");
  const [open, setOpen] = useState(false);
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="ghost" size="sm" />}>
        <GitBranch />
        {t(($) => $.infrastructure.service_relations)}
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{`${props.service.name} · ${t(($) => $.infrastructure.service_relations)}`}</DialogTitle>
          <DialogDescription>
            {t(($) => $.infrastructure.dependency_direction)}
          </DialogDescription>
        </DialogHeader>
        {open && <ServiceRelations {...props} />}
      </DialogContent>
    </Dialog>
  );
}

function ServiceRelations({
  ws,
  connection,
  cluster,
  service,
  services,
}: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
  service: InfrastructureService;
  services: InfrastructureService[];
}) {
  const { t } = useT("runtimes");
  const p = useWorkspacePaths();
  const relations = useQuery({
    queryKey: [
      ...resourceKey(ws, connection.id),
      cluster,
      service.id,
      "relations",
    ],
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "service_relations",
        { connection_id: connection.id, service_id: service.id },
        InfrastructureServiceRelationsSchema,
      ),
  });
  const column = (label: string, rows: InfrastructureService[]) => (
    <div className="grid min-w-0 content-start gap-2">
      <h4 className="text-caption text-muted-foreground">{label}</h4>
      {rows.map((row) => {
        const local = services.some((s) => s.id === row.id);
        const content = (
          <>
            <span className="block break-words font-medium">{row.name}</span>
            <span className="block text-caption text-muted-foreground">
              {row.cluster_id === 0
                ? t(($) => $.infrastructure.global_service)
                : row.cluster_id !== cluster
                  ? row.cluster_name || `#${row.cluster_id}`
                  : row.type_name || row.type}
            </span>
          </>
        );
        return local ? (
          <AppLink
            key={row.id}
            href={clusterHref(
              p.infrastructure(),
              connection.id,
              cluster,
              row.id,
            )}
            className="rounded-md border px-3 py-2 text-body hover:bg-surface-hover"
          >
            {content}
          </AppLink>
        ) : (
          <div
            key={row.id}
            className="rounded-md border bg-muted/20 px-3 py-2 text-body"
          >
            {content}
          </div>
        );
      })}
      {rows.length === 0 && (
        <span className="text-caption text-muted-foreground">
          {t(($) => $.infrastructure.no_relations)}
        </span>
      )}
    </div>
  );
  return (
    <section className="grid min-w-0 gap-3">
      <ErrorMessage error={relations.error} />
      {relations.isPending && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.infrastructure.loading)}
        </p>
      )}
      {relations.data && (
        <div className="grid min-w-0 gap-3 md:grid-cols-[1fr_auto_1fr_auto_1fr]">
          {column(
            t(($) => $.infrastructure.dependents),
            relations.data.dependents,
          )}
          <ArrowRight className="mt-8 size-4 self-start text-muted-foreground max-md:rotate-90" />
          <div className="grid min-w-0 content-start gap-2">
            <h4 className="text-caption text-muted-foreground">
              {t(($) => $.infrastructure.service)}
            </h4>
            <div className="break-words rounded-md border bg-surface-selected px-3 py-2 text-body font-medium">
              {service.name}
            </div>
          </div>
          <ArrowRight className="mt-8 size-4 self-start text-muted-foreground max-md:rotate-90" />
          {column(
            t(($) => $.infrastructure.dependencies),
            relations.data.dependencies,
          )}
        </div>
      )}
      {service.external_dependencies.length > 0 && (
        <p className="break-words text-caption text-muted-foreground">{`${t(($) => $.infrastructure.external_dependencies)}: ${service.external_dependencies.join(", ")}`}</p>
      )}
    </section>
  );
}
