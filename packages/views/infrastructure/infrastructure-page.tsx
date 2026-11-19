"use client";

import { useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  InfrastructureNodeSchema,
  infrastructureConnectionsOptions,
  infrastructureClustersOptions,
  infrastructureServicesOptions,
  infrastructureResourceKey,
  infrastructureHref,
  groupInfrastructureProducts,
  type InfrastructureConnection,
  type InfrastructureService,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { ArrowLeft, ChevronRight, RefreshCw, Server } from "@multica/ui/icons";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { ErrorMessage, ResourceTable } from "./components";
import { ServiceCatalog } from "./service-catalog";
import { ServiceRelationsDialog } from "./service-relations-dialog";
import { ServiceResources } from "./service-resources";
import { ServiceMetainfo } from "./service-metainfo";
import { OperationHistory } from "./operation-history";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";

export function InfrastructurePage() {
  const { t } = useT("runtimes");
  const ws = useWorkspaceId();
  const q = useQueryClient();
  const nav = useInfrastructureNavigation();
  const p = useWorkspacePaths();
  const connections = useQuery(infrastructureConnectionsOptions(ws));
  const connection =
    connections.data?.connections.find(
      (c) => c.id === nav.searchParams.get("manager"),
    ) ?? connections.data?.connections[0];
  const clusters = useQuery(
    infrastructureClustersOptions(ws, connection?.id ?? ""),
  );
  const cluster =
    clusters.data?.find(
      (c) => String(c.id) === nav.searchParams.get("cluster"),
    ) ?? clusters.data?.[0];
  const services = useQuery(
    infrastructureServicesOptions(ws, connection?.id ?? "", cluster?.id ?? 0),
  );
  const serviceId = nav.searchParams.get("service");
  const service = serviceId
    ? services.data?.find((s) => String(s.id) === serviceId)
    : undefined;
  const href = (patch: Record<string, string | number | null>) =>
    infrastructureHref(p.infrastructure(), nav.searchParams, patch);
  const listHref = href({ service: null, tab: null, hostname: null });
  const view =
    nav.searchParams.get("view") === "nodes"
      ? "nodes"
      : nav.searchParams.get("view") === "history"
        ? "history"
        : "services";
  const products = groupInfrastructureProducts(services.data ?? []).filter(
    (group) => group.product !== null,
  );
  const versionCount = products.reduce(
    (count, group) => count + group.versions.length,
    0,
  );
  useEffect(() => {
    if (
      connection &&
      cluster &&
      (nav.searchParams.get("manager") !== connection.id ||
        nav.searchParams.get("cluster") !== String(cluster.id))
    ) {
      nav.replace(
        infrastructureHref(p.infrastructure(), nav.searchParams, {
          manager: connection.id,
          cluster: cluster.id,
        }),
      );
    }
  }, [connection, cluster, nav, p]);

  return (
    <section
      aria-label={t(($) => $.infrastructure.title)}
      className="flex h-full min-h-0 flex-col"
    >
      <header className="flex h-12 shrink-0 items-center justify-between gap-3 border-b px-4 sm:px-7">
        <nav
          aria-label={t(($) => $.infrastructure.title)}
          className="flex min-w-0 items-center gap-2 text-caption text-muted-foreground"
        >
          <span className="shrink-0">{t(($) => $.infrastructure.title)}</span>
          {connection && (
            <>
              <ChevronRight className="size-3 shrink-0" aria-hidden="true" />
              <span className="truncate" title={connection.url}>
                {connection.name}
              </span>
            </>
          )}
          {cluster && (
            <>
              <ChevronRight className="size-3 shrink-0" aria-hidden="true" />
              <AppLink
                href={listHref}
                className="truncate rounded-sm hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
              >
                {cluster.name}
              </AppLink>
            </>
          )}
          {service && (
            <>
              <ChevronRight className="size-3 shrink-0" aria-hidden="true" />
              <span className="truncate text-foreground">{service.name}</span>
            </>
          )}
        </nav>
        {serviceId && (
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(($) => $.infrastructure.refresh)}
            onClick={() =>
              q.invalidateQueries({ queryKey: ["infrastructure", ws] })
            }
          >
            <RefreshCw />
          </Button>
        )}
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <div className="mx-auto flex w-full max-w-[100rem] min-w-0 flex-col gap-5 px-4 py-6 sm:px-8">
          <ErrorMessage
            error={connections.error ?? clusters.error ?? services.error}
          />
          {(connections.isPending || (connection && clusters.isPending)) && (
            <p role="status">{t(($) => $.infrastructure.loading)}</p>
          )}
          {connections.data && !connections.data.configured && (
            <p role="status" className="rounded-lg border p-4 text-body">
              {t(($) => $.infrastructure.setup)}
            </p>
          )}
          {connection && cluster ? (
            serviceId ? (
              <>
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <AppLink
                    href={listHref}
                    className="inline-flex items-center gap-2 rounded-sm text-caption text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
                  >
                    <ArrowLeft className="size-3.5" />
                    {t(($) => $.infrastructure.back_to_services)}
                  </AppLink>
                  {service && (
                    <ServiceRelationsDialog
                      ws={ws}
                      connection={connection}
                      cluster={cluster.id}
                      service={service}
                      services={services.data ?? []}
                    />
                  )}
                </div>
                {service ? (
                  <ServiceDetail
                    key={`${ws}:${connection.id}:${cluster.id}:${service.id}`}
                    ws={ws}
                    connection={connection}
                    cluster={cluster.id}
                    service={service}
                    canManage={connections.data?.can_manage === true}
                  />
                ) : (
                  <p role="status">
                    {t(($) =>
                      services.isPending
                        ? $.infrastructure.loading
                        : $.infrastructure.service_unavailable,
                    )}
                  </p>
                )}
              </>
            ) : (
              <>
                <div className="flex flex-wrap items-center justify-between gap-3 pb-1">
                  <div className="grid gap-2">
                    <h1 className="text-heading font-semibold">
                      {cluster.name}
                    </h1>
                    {services.data && (
                      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-caption text-muted-foreground">
                        <span>
                          {t(($) => $.infrastructure.services_count, {
                            count: services.data.length,
                          })}
                        </span>
                        {products.length > 0 && (
                          <>
                            <span aria-hidden="true">·</span>
                            <span>
                              {t(
                                ($) => $.infrastructure.catalog.products_count,
                                { count: products.length },
                              )}
                            </span>
                            <span aria-hidden="true">·</span>
                            <span>
                              {t(
                                ($) => $.infrastructure.catalog.versions_count,
                                { count: versionCount },
                              )}
                            </span>
                          </>
                        )}
                      </div>
                    )}
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      q.invalidateQueries({ queryKey: ["infrastructure", ws] })
                    }
                  >
                    <RefreshCw className="size-3.5" />
                    {t(($) => $.infrastructure.refresh)}
                  </Button>
                </div>
                <InfrastructureTabs
                  label={t(($) => $.infrastructure.cluster)}
                  value={view}
                  items={[
                    {
                      value: "services",
                      label: t(($) => $.infrastructure.services),
                      href: href({ view: null }),
                    },
                    {
                      value: "nodes",
                      label: t(($) => $.infrastructure.nodes),
                      href: href({ view: "nodes" }),
                    },
                    {
                      value: "history",
                      label: t(($) => $.infrastructure.operations),
                      href: href({ view: "history" }),
                    },
                  ]}
                />
                {view === "services" ? (
                  <ServiceCatalog
                    key={`${ws}:${connection.id}:${cluster.id}`}
                    services={services.data ?? []}
                    loading={services.isPending}
                  />
                ) : view === "nodes" ? (
                  <ClusterNodes
                    ws={ws}
                    connection={connection}
                    cluster={cluster.id}
                  />
                ) : (
                  <OperationHistory
                    ws={ws}
                    connection={connection}
                    cluster={cluster.id}
                  />
                )}
              </>
            )
          ) : (
            connections.data &&
            !clusters.isPending && (
              <p className="py-12 text-center text-body text-muted-foreground">
                {t(($) =>
                  connections.data.connections.length
                    ? $.infrastructure.no_clusters
                    : $.infrastructure.empty,
                )}
              </p>
            )
          )}
        </div>
      </div>
    </section>
  );
}

export function InfrastructureTabs({
  label,
  value,
  items,
}: {
  label: string;
  value: string;
  items: { value: string; label: string; href: string }[];
}) {
  return (
    <nav
      aria-label={label}
      className="flex shrink-0 gap-6 overflow-x-auto border-b"
    >
      {items.map((item) => (
        <AppLink
          key={item.value}
          href={item.href}
          aria-current={value === item.value ? "page" : undefined}
          className={cn(
            "-mb-px whitespace-nowrap border-b-2 px-0.5 pb-3 pt-1 text-body outline-none focus-visible:outline-2 focus-visible:outline-ring",
            value === item.value
              ? "border-foreground font-medium text-foreground hover:text-foreground"
              : "border-transparent text-muted-foreground hover:text-foreground",
          )}
        >
          {item.label}
        </AppLink>
      ))}
    </nav>
  );
}

function ServiceDetail({
  ws,
  connection,
  cluster,
  service,
  canManage,
}: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
  service: InfrastructureService;
  canManage: boolean;
}) {
  const { t } = useT("runtimes");
  const nav = useInfrastructureNavigation();
  const p = useWorkspacePaths();
  const tab =
    nav.searchParams.get("tab") === "logs"
      ? "logs"
      : nav.searchParams.get("tab") === "metainfo"
        ? "metainfo"
        : "roles";
  const href = (value: string) =>
    infrastructureHref(p.infrastructure(), nav.searchParams, { tab: value });
  return (
    <>
      <div className="flex min-w-0 items-start gap-3 py-1">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-brand/10 text-brand">
          <Server className="size-5" />
        </div>
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="break-words text-heading font-semibold">
              {service.name}
            </h1>
            <span
              className={cn(
                "text-caption",
                service.health === "HEALTHY"
                  ? "text-success"
                  : service.health === "DOWN"
                    ? "text-destructive"
                    : "text-muted-foreground",
              )}
            >
              {service.health}
            </span>
          </div>
          <p className="mt-1 break-words text-caption text-muted-foreground">
            {[
              service.product?.name,
              service.type_name || service.type,
              service.version,
            ]
              .filter(Boolean)
              .join(" / ")}
          </p>
        </div>
      </div>
      <InfrastructureTabs
        label={t(($) => $.infrastructure.service)}
        value={tab}
        items={[
          {
            value: "roles",
            label: t(($) => $.infrastructure.roles_tab),
            href: href("roles"),
          },
          {
            value: "logs",
            label: t(($) => $.infrastructure.logs),
            href: href("logs"),
          },
          {
            value: "metainfo",
            label: t(($) => $.infrastructure.metainfo),
            href: href("metainfo"),
          },
        ]}
      />
      {tab === "metainfo" ? (
        <ServiceMetainfo
          ws={ws}
          connection={connection}
          service={service}
          canManage={canManage}
        />
      ) : (
        <ServiceResources
          ws={ws}
          connection={connection}
          cluster={cluster}
          service={service.id}
          canManage={canManage}
          tab={tab}
        />
      )}
    </>
  );
}

function ClusterNodes({
  ws,
  connection,
  cluster,
}: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
}) {
  const { t } = useT("runtimes");
  const nodes = useQuery({
    queryKey: [
      ...infrastructureResourceKey(ws, connection.id),
      cluster,
      "nodes",
    ],
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "list_nodes",
        { connection_id: connection.id, cluster_id: cluster },
        InfrastructureNodeSchema.array(),
      ),
  });
  return (
    <section className="grid gap-3">
      <ErrorMessage error={nodes.error} />
      <ResourceTable
        headings={[
          t(($) => $.infrastructure.node),
          t(($) => $.infrastructure.ip),
          t(($) => $.infrastructure.status),
          t(($) => $.infrastructure.cores),
          t(($) => $.infrastructure.memory),
          t(($) => $.infrastructure.disk),
        ]}
        rows={(nodes.data ?? []).map((n) => [
          n.name,
          n.ip,
          n.status,
          n.cores,
          n.memory.toLocaleString(),
          n.disk.toLocaleString(),
        ])}
        empty={t(($) =>
          nodes.isPending
            ? $.infrastructure.loading
            : $.infrastructure.no_results,
        )}
      />
    </section>
  );
}
