"use client";
import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  InfrastructureRoleSchema,
  InfrastructureLogSchema,
  InfrastructureOperationSchema,
  infrastructureConnectionsKey as connectionKey,
  infrastructureResourceKey as resourceKey,
  infrastructureHref,
  type InfrastructureConnection,
  type InfrastructureRole,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../i18n";
import { AppLink } from "../navigation";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";
import { ErrorMessage, Field, Picker, ResourceTable } from "./components";

export function ServiceResources({
  ws,
  connection,
  cluster,
  service,
  canManage,
  tab,
}: {
  ws: string;
  connection: InfrastructureConnection;
  cluster: number;
  service: number;
  canManage: boolean;
  tab: "roles" | "logs";
}) {
  const { t } = useT("runtimes");
  const q = useQueryClient();
  const nav = useInfrastructureNavigation();
  const p = useWorkspacePaths();
  const base = {
    connection_id: connection.id,
    cluster_id: cluster,
    service_id: service,
  };
  const key = [...resourceKey(ws, connection.id), cluster, service];
  const roles = useQuery({
    queryKey: [...key, "roles"],
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "list_roles",
        base,
        InfrastructureRoleSchema.array(),
      ),
  });
  const grant = useMutation({
    mutationFn: ({
      role,
      enabled,
    }: {
      role: InfrastructureRole;
      enabled: boolean;
    }) =>
      api.setInfrastructureGrants(ws, connection.id, [
        ...connection.restart_grants.filter((g) => g.role_id !== role.id),
        ...(enabled
          ? [{ cluster_id: cluster, service_id: service, role_id: role.id }]
          : []),
      ]),
    onSuccess: () => q.invalidateQueries({ queryKey: connectionKey(ws) }),
  });
  const requestedHost = nav.searchParams.get("hostname") ?? "";
  const [host, setHost] = useState(requestedHost);
  const [level, setLevel] = useState("ERROR");
  const [keywords, setKeywords] = useState("");
  const [search, setSearch] = useState<{
    hostname: string;
    level: string;
    keywords: string;
  } | null>(null);
  useEffect(() => {
    setHost(requestedHost);
    if (requestedHost)
      setSearch({ hostname: requestedHost, level: "ERROR", keywords: "" });
  }, [requestedHost]);
  const logs = useQuery({
    queryKey: [...key, "logs", search],
    queryFn: () =>
      api.infrastructureQuery(
        ws,
        "search_logs",
        { ...base, ...search },
        InfrastructureLogSchema.array(),
      ),
    enabled: tab === "logs" && !!search,
  });
  return (
    <>
      {tab === "roles" && (
        <section className="grid gap-3">
          <div className="flex items-center gap-2">
            <h2 className="text-subheading">
              {t(($) => $.infrastructure.roles)}
            </h2>
            {roles.data && (
              <span className="text-caption text-muted-foreground">
                {roles.data.length}
              </span>
            )}
          </div>
          <ErrorMessage error={roles.error ?? grant.error} />
          <ResourceTable
            headings={[
              t(($) => $.infrastructure.roles),
              t(($) => $.infrastructure.node),
              t(($) => $.infrastructure.status),
              t(($) => $.infrastructure.health),
              t(($) => $.infrastructure.automatic_restart),
              t(($) => $.infrastructure.logs),
              t(($) => $.infrastructure.restart),
            ]}
            rows={(roles.data ?? []).map((role) => {
              const allowed = connection.restart_grants.some(
                (g) =>
                  g.cluster_id === cluster &&
                  g.service_id === service &&
                  g.role_id === role.id,
              );
              return [
                role.name || role.type,
                role.node.name,
                role.status,
                role.health,
                <Dialog key="grant">
                  <DialogTrigger
                    render={
                      <Button
                        variant="outline"
                        size="xs"
                        disabled={!canManage || !role.installed}
                      />
                    }
                  >
                    {t(($) =>
                      allowed
                        ? $.infrastructure.role_allowed
                        : $.infrastructure.role_not_allowed,
                    )}
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>
                        {t(($) => $.infrastructure.allow_restart)}
                      </DialogTitle>
                      <DialogDescription>
                        {role.name || role.type} · {role.node.name}
                      </DialogDescription>
                    </DialogHeader>
                    <label className="flex items-center gap-2 text-body">
                      <input
                        type="checkbox"
                        aria-label={`${t(($) => $.infrastructure.allow_restart)} · ${role.name || role.type}`}
                        checked={allowed}
                        disabled={grant.isPending}
                        onChange={(event) =>
                          grant.mutate({ role, enabled: event.target.checked })
                        }
                      />
                      {t(($) => $.infrastructure.allow_restart)}
                    </label>
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.infrastructure.restart_hint)}
                    </p>
                    <ErrorMessage error={grant.error} />
                  </DialogContent>
                </Dialog>,
                <AppLink
                  key="logs"
                  href={infrastructureHref(
                    p.infrastructure(),
                    nav.searchParams,
                    { tab: "logs", hostname: role.node.name },
                  )}
                  className="whitespace-nowrap rounded-sm text-caption text-brand hover:underline focus-visible:outline-2 focus-visible:outline-ring"
                >
                  {t(($) => $.infrastructure.view_logs)}
                </AppLink>,
                <RestartRole
                  key={role.id}
                  ws={ws}
                  connection={connection}
                  role={role}
                  cluster={cluster}
                  service={service}
                  allowed={allowed}
                />,
              ];
            })}
            empty={t(($) =>
              roles.isPending
                ? $.infrastructure.loading
                : $.infrastructure.no_results,
            )}
          />
          <p className="text-caption text-muted-foreground">
            {t(($) => $.infrastructure.restart_hint)}
          </p>
        </section>
      )}
      {tab === "logs" && (
        <section className="grid gap-3">
          <h2 className="text-subheading">{t(($) => $.infrastructure.logs)}</h2>
          <form
            className="flex flex-wrap items-end gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              const next = { hostname: host, level, keywords };
              if (JSON.stringify(next) === JSON.stringify(search))
                void logs.refetch();
              else setSearch(next);
            }}
          >
            <Picker
              label={t(($) => $.infrastructure.hostname)}
              value={host}
              items={Array.from(
                new Set((roles.data ?? []).map((r) => r.node.name)),
              )
                .filter(Boolean)
                .map((name) => ({ value: name, label: name }))}
              onChange={setHost}
            />
            <Picker
              label={t(($) => $.infrastructure.level)}
              value={level}
              items={["FATAL", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"].map(
                (value) => ({ value, label: value }),
              )}
              onChange={setLevel}
            />
            <Field label={t(($) => $.infrastructure.keywords)}>
              <Input
                value={keywords}
                onChange={(e) => setKeywords(e.target.value)}
              />
            </Field>
            <Button type="submit" disabled={!host || logs.isFetching}>
              {t(($) => $.infrastructure.search)}
            </Button>
          </form>
          <ErrorMessage error={logs.error} />
          {logs.data && (
            <div className="max-h-96 overflow-auto rounded-lg border bg-muted/30 p-4">
              <pre className="whitespace-pre-wrap break-all text-caption">
                {logs.data.length
                  ? logs.data
                      .map(
                        (l) =>
                          `${l.time} ${l.level} ${l.hostname} ${l.message}`,
                      )
                      .join("\n")
                  : t(($) => $.infrastructure.no_results)}
              </pre>
            </div>
          )}
        </section>
      )}
    </>
  );
}

function RestartRole({
  ws,
  connection,
  role,
  cluster,
  service,
  allowed,
}: {
  ws: string;
  connection: InfrastructureConnection;
  role: InfrastructureRole;
  cluster: number;
  service: number;
  allowed: boolean;
}) {
  const { t } = useT("runtimes");
  const q = useQueryClient();
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const restart = useMutation({
    retry: false,
    mutationFn: () =>
      api.infrastructureQuery(
        ws,
        "restart_role",
        {
          connection_id: connection.id,
          cluster_id: cluster,
          service_id: service,
          role_id: role.id,
          reason,
        },
        InfrastructureOperationSchema,
      ),
    onSuccess: async (result) => {
      q.setQueryData(
        [...resourceKey(ws, connection.id), "operation", result.id],
        result,
      );
      await q.invalidateQueries({
        queryKey: [...resourceKey(ws, connection.id), "operations"],
      });
      setOpen(false);
      setReason("");
    },
  });
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!restart.isPending) setOpen(value);
      }}
    >
      <DialogTrigger
        render={
          <Button
            size="sm"
            variant="outline"
            disabled={!allowed || restart.isPending}
          />
        }
      >
        {t(($) => $.infrastructure.restart)}
      </DialogTrigger>
      <DialogContent>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            restart.mutate();
          }}
        >
          <DialogHeader>
            <DialogTitle>
              {t(($) => $.infrastructure.restart)} · {role.name || role.type}
            </DialogTitle>
            <DialogDescription>{`${connection.name} · #${cluster} / #${service} / #${role.id}`}</DialogDescription>
          </DialogHeader>
          <Field label={t(($) => $.infrastructure.reason)}>
            <Textarea
              required
              maxLength={2000}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </Field>
          <ErrorMessage error={restart.error} />
          <DialogFooter>
            <DialogClose
              render={<Button variant="outline" disabled={restart.isPending} />}
            >
              {t(($) => $.infrastructure.cancel)}
            </DialogClose>
            <Button
              type="submit"
              disabled={!reason.trim() || restart.isPending}
              aria-busy={restart.isPending}
            >
              {t(($) => $.infrastructure.restart)}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
