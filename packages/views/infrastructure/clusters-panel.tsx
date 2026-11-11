"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Search, Server, Settings } from "@multica/ui/icons";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  clusterHref,
  infrastructureConnectionsOptions,
  infrastructureClustersOptions,
} from "@multica/core/infrastructure";
import { useT } from "../i18n";
import {
  PanelHeader,
  PanelBody,
  PanelList,
  PanelLink,
} from "../layout/panel-nav";
import { ConnectionDialog, RemoveConnection } from "./connection-dialogs";
import { ErrorMessage } from "./components";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";

export function ClustersPanel() {
  const ws = useWorkspaceId();
  const { t } = useT("runtimes");
  const p = useWorkspacePaths();
  const nav = useInfrastructureNavigation();
  const [search, setSearch] = useState("");
  const connections = useQuery(infrastructureConnectionsOptions(ws));
  const connection =
    connections.data?.connections.find(
      (c) => c.id === nav.searchParams.get("manager"),
    ) ?? connections.data?.connections[0];
  const clusters = useQuery(
    infrastructureClustersOptions(ws, connection?.id ?? ""),
  );
  const items = (connections.data?.connections ?? []).map((c) => ({
    value: c.id,
    label: c.name,
  }));
  return (
    <>
      <PanelHeader title={t(($) => $.infrastructure.title)} />
      <div className="grid gap-2 px-4 pb-5 pt-2">
        <span className="text-caption text-muted-foreground">
          {t(($) => $.infrastructure.manager)}
        </span>
        <Select
          items={items}
          value={connection?.id ?? null}
          onValueChange={(value) => {
            if (value && value !== connection?.id) {
              setSearch("");
              nav.push(
                `${p.infrastructure()}?${new URLSearchParams({ manager: value })}`,
              );
            }
          }}
        >
          <SelectTrigger
            aria-label={t(($) => $.infrastructure.switch_manager)}
            className="w-full"
          >
            <SelectValue placeholder={t(($) => $.infrastructure.manager)} />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {connection && (
          <p
            className="truncate text-caption text-muted-foreground"
            title={connection.url}
          >
            {connection.url}
          </p>
        )}
      </div>
      <div className="relative mx-4 mb-4">
        <Search className="pointer-events-none absolute left-2.5 top-2.5 size-3.5 text-muted-foreground" />
        <Input
          aria-label={t(($) => $.infrastructure.search_clusters)}
          placeholder={t(($) => $.infrastructure.search_clusters)}
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          className="pl-8"
        />
      </div>
      <PanelBody>
        <ErrorMessage error={connections.error ?? clusters.error} />
        <p className="px-3 pb-2 text-caption text-muted-foreground">
          {t(($) => $.infrastructure.cluster_list)}
        </p>
        {(connections.isPending || (connection && clusters.isPending)) && (
          <p className="px-3 py-4 text-caption text-muted-foreground">
            {t(($) => $.infrastructure.loading)}
          </p>
        )}
        <PanelList>
          {clusters.data
            ?.filter((c) =>
              c.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()),
            )
            .map((cluster, index) => (
              <PanelLink
                key={cluster.id}
                href={clusterHref(
                  p.infrastructure(),
                  connection!.id,
                  cluster.id,
                )}
                label={cluster.name}
                icon={<Server />}
                active={
                  nav.searchParams.get("cluster") === String(cluster.id) ||
                  (!nav.searchParams.get("cluster") && index === 0)
                }
              />
            ))}
        </PanelList>
        {clusters.data &&
          !clusters.data.some((c) =>
            c.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()),
          ) && (
            <p className="px-3 py-4 text-caption text-muted-foreground">
              {t(($) =>
                clusters.data.length
                  ? $.infrastructure.no_results
                  : $.infrastructure.no_clusters,
              )}
            </p>
          )}
        {clusters.isError && (
          <Button variant="ghost" size="sm" onClick={() => clusters.refetch()}>
            {t(($) => $.infrastructure.refresh)}
          </Button>
        )}
      </PanelBody>
      {connections.data?.can_manage && (
        <div className="shrink-0 p-3">
          <Dialog>
            <DialogTrigger
              render={
                <Button
                  variant="ghost"
                  className="w-full justify-start text-muted-foreground"
                />
              }
            >
              <Settings />
              {t(($) => $.infrastructure.manage_managers)}
            </DialogTrigger>
            <DialogContent
              className="max-h-[85dvh] overflow-y-auto sm:max-w-2xl"
              aria-describedby={undefined}
            >
              <DialogHeader>
                <DialogTitle>
                  {t(($) => $.infrastructure.manage_managers)}
                </DialogTitle>
              </DialogHeader>
              <div className="grid gap-3">
                {connections.data.connections.map((c) => (
                  <div
                    key={c.id}
                    className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3"
                  >
                    <div className="min-w-0">
                      <p className="font-medium">{c.name}</p>
                      <p className="break-all text-caption text-muted-foreground">
                        {c.url}
                      </p>
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <ConnectionDialog ws={ws} connection={c} />
                      <RemoveConnection ws={ws} connection={c} />
                    </div>
                  </div>
                ))}
              </div>
              <div className="flex justify-end">
                <ConnectionDialog
                  ws={ws}
                  disabled={!connections.data.configured}
                />
              </div>
            </DialogContent>
          </Dialog>
        </div>
      )}
    </>
  );
}
