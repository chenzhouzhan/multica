"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import { useWorkspacePaths } from "@multica/core/paths";
import {
  infrastructureHref,
  infrastructureImagesOptions,
  infrastructureImagesKey,
  infrastructureImageOperationOptions,
  infrastructureResourceKey,
  type InfrastructureImageScope,
  type InfrastructureImagePreviewInput,
  type InfrastructureOperation,
  type InfrastructureService,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { useT } from "../i18n";
import { ErrorMessage, Field } from "./components";
import { ExecutionAccessDialog } from "./execution-access-dialog";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";

export function ImageOperationDetails({
  operation,
}: {
  operation: InfrastructureOperation;
}) {
  const { t } = useT("runtimes");
  return (
    <div className="grid gap-4">
      <p role="status" className="text-body font-medium">
        {t(($) => $.infrastructure.image_release.status[operation.status])}
      </p>
      {operation.details?.items.map((item) => (
        <div
          key={item.target.key}
          className="grid min-w-0 gap-2 rounded-lg border p-4 text-body"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="font-medium">
              {item.target.role_name || item.target.container}
            </h3>
            <span className="text-caption">
              {t(($) => $.infrastructure.image_release.status[item.status])}
            </span>
          </div>
          <p className="break-all text-caption text-muted-foreground">
            {item.target.namespace} / {item.target.workload} /{" "}
            {item.target.container}
          </p>
          <div className="grid gap-1">
            <p className="text-caption text-muted-foreground">
              {t(($) => $.infrastructure.image_release.current_image)}
            </p>
            <code className="break-all text-caption">{item.target.image}</code>
          </div>
          {operation.kind !== "restart_pods" && (
            <div className="grid gap-1">
              <p className="text-caption text-muted-foreground">
                {t(($) => $.infrastructure.image_release.source, {
                  role: item.target.role_name || item.target.container,
                })}
              </p>
              <code className="break-all text-caption">
                {item.image.source}
              </code>
            </div>
          )}
          {item.image.pinned && (
            <div className="grid gap-1">
              <p className="text-caption text-muted-foreground">
                {t(($) => $.infrastructure.image_release.digest)}
              </p>
              <code className="break-all text-caption">
                {item.image.pinned}
              </code>
            </div>
          )}
        </div>
      ))}
      {operation.details?.steps.map((step, index) => (
        <div
          key={`${index}:${step.target_key}`}
          className="grid gap-1 text-caption"
        >
          <div className="flex flex-wrap justify-between gap-2">
            <span>
              {index + 1}.{" "}
              {t(($) => $.infrastructure.image_release.step[step.kind])}
              {step.target_key &&
                ` · ${operation.details?.items.find((item) => item.target.key === step.target_key)?.target.role_name || step.target_key}`}
            </span>
            <span>
              {t(($) => $.infrastructure.image_release.status[step.status])}
            </span>
          </div>
          <ErrorMessage error={step.error} />
        </div>
      ))}
      <ErrorMessage
        error={
          operation.details?.steps.some(
            (step) => step.error === operation.error,
          )
            ? null
            : operation.error
        }
      />
      {operation.status === "unknown" && (
        <p className="text-caption text-warning">
          {t(($) => $.infrastructure.image_release.uncertain)}
        </p>
      )}
      {operation.status === "verification_required" && (
        <p className="text-caption text-warning">
          {t(($) => $.infrastructure.image_release.verification)}
        </p>
      )}
    </div>
  );
}

export function ServiceImages({
  scope,
  service,
  canManage,
}: {
  scope: InfrastructureImageScope;
  service: InfrastructureService;
  canManage: boolean;
}) {
  const { t } = useT("runtimes");
  const nav = useInfrastructureNavigation();
  const p = useWorkspacePaths();
  const q = useQueryClient();
  const resources = useQuery(infrastructureImagesOptions(scope));
  const [action, setAction] =
    useState<InfrastructureImagePreviewInput["action"]>("publish_images");
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [sources, setSources] = useState<Record<string, string>>({});
  const [reason, setReason] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const initializedRole = useRef<string | null>(null);
  const requestedRole = nav.searchParams.get("role_type") ?? "";
  const operationId = nav.searchParams.get("image_operation") ?? "";
  const operation = useQuery(
    infrastructureImageOperationOptions(
      scope.ws,
      scope.connection,
      operationId,
    ),
  );
  const current =
    operation.data?.connection_id === scope.connection &&
    operation.data.cluster_id === scope.cluster &&
    operation.data.service_id === scope.service
      ? operation.data
      : undefined;
  const active =
    current?.status === "preparing" || current?.status === "running";
  const currentOperationId = current?.id;
  const refetchOperation = operation.refetch;
  useEffect(() => {
    if (!resources.data || initializedRole.current === requestedRole) return;
    initializedRole.current = requestedRole;
    if (requestedRole)
      setSelected(
        Object.fromEntries(
          resources.data.targets
            .filter(
              (target) =>
                target.role_type === requestedRole && !target.blocked_reason,
            )
            .map((target) => [target.key, true]),
        ),
      );
  }, [requestedRole, resources.data]);
  useEffect(() => {
    if (!currentOperationId || !active) return;
    const timer = setInterval(() => {
      void refetchOperation();
    }, 3000);
    return () => clearInterval(timer);
  }, [active, currentOperationId, refetchOperation]);
  const settled = useRef("");
  useEffect(() => {
    if (
      !current ||
      ["preparing", "ready", "running"].includes(current.status) ||
      settled.current === current.id + current.status
    )
      return;
    settled.current = current.id + current.status;
    void q.invalidateQueries({ queryKey: infrastructureImagesKey(scope) });
    void q.invalidateQueries({
      queryKey: [
        ...infrastructureResourceKey(scope.ws, scope.connection),
        "operations",
      ],
    });
  }, [current, q, scope]);
  const openOperation = (result: InfrastructureOperation) => {
    q.setQueryData(
      infrastructureImageOperationOptions(scope.ws, scope.connection, result.id)
        .queryKey,
      result,
    );
    nav.replace(
      infrastructureHref(p.infrastructure(), nav.searchParams, {
        tab: "images",
        image_operation: result.id,
        role_type: null,
      }),
    );
  };
  const preview = useMutation({
    retry: false,
    mutationFn: () =>
      api.previewInfrastructureImages(scope, {
        action,
        reason,
        items: (resources.data?.targets ?? [])
          .filter((target) => selected[target.key] && !target.blocked_reason)
          .map((target) => ({
            key: target.key,
            source: sources[target.key]?.trim() ?? "",
          })),
      }),
    onSuccess: openOperation,
  });
  const start = useMutation({
    retry: false,
    mutationFn: () => api.startInfrastructureImages(scope, operationId),
    onSuccess: openOperation,
    onError: () => {
      void refetchOperation();
    },
  });
  const resetSubmission = start.reset;
  useEffect(() => {
    setConfirmed(false);
    resetSubmission();
  }, [operationId, resetSubmission]);
  const count = (resources.data?.targets ?? []).filter(
    (target) => selected[target.key] && !target.blocked_reason,
  ).length;
  const ready =
    current?.status === "ready" &&
    current.details !== null &&
    current.details.items.length > 0;
  const reset = () =>
    nav.replace(
      infrastructureHref(p.infrastructure(), nav.searchParams, {
        image_operation: null,
      }),
    );
  return (
    <section className="grid gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-subheading">
          {t(($) => $.infrastructure.image_release.title)}
        </h2>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={resources.isFetching}
            onClick={() => {
              void resources.refetch();
              if (operationId) void refetchOperation();
            }}
          >
            {t(($) => $.infrastructure.refresh)}
          </Button>
          {canManage && (
            <ExecutionAccessDialog
              ws={scope.ws}
              connection={scope.connection}
              cluster={scope.cluster}
            />
          )}
        </div>
      </div>
      <ErrorMessage error={resources.error || operation.error} />
      {!canManage && (
        <p className="text-caption text-muted-foreground">
          {t(($) => $.infrastructure.image_release.read_only)}
        </p>
      )}
      {resources.isPending && (
        <p role="status">{t(($) => $.infrastructure.loading)}</p>
      )}
      {resources.data && !resources.data.configured && (
        <p className="text-body text-muted-foreground">
          {t(($) => $.infrastructure.image_release.setup)}
        </p>
      )}
      {operationId ? (
        <div className="grid gap-4">
          {current ? (
            <>
              <ImageOperationDetails operation={current} />
              {current.status === "preparing" && (
                <p role="status">
                  {t(($) => $.infrastructure.image_release.preparing)}
                </p>
              )}
              {ready && canManage && (
                <label className="flex items-start gap-2 text-body">
                  <input
                    type="checkbox"
                    checked={confirmed}
                    onChange={(event) => setConfirmed(event.target.checked)}
                  />
                  {t(($) => $.infrastructure.image_release.confirm_scope, {
                    service: service.name,
                  })}
                </label>
              )}
              {current.status === "ready" && !ready && (
                <p role="alert">
                  {t(($) => $.infrastructure.image_release.invalid_preview)}
                </p>
              )}
              <ErrorMessage error={start.error} />
              <div className="flex flex-wrap justify-end gap-2">
                <Button
                  variant="outline"
                  disabled={active || start.isPending}
                  onClick={reset}
                >
                  {t(($) => $.infrastructure.image_release.back)}
                </Button>
                {ready && canManage && (
                  <Button
                    disabled={!confirmed || start.isPending || start.isError}
                    aria-busy={start.isPending}
                    onClick={() => start.mutate()}
                  >
                    {current.kind === "restart_pods"
                      ? t(($) => $.infrastructure.image_release.restart)
                      : t(($) => $.infrastructure.image_release.confirm)}
                  </Button>
                )}
              </div>
            </>
          ) : (
            <p role="status">
              {operation.isPending
                ? t(($) => $.infrastructure.loading)
                : t(($) => $.infrastructure.image_release.invalid_preview)}
            </p>
          )}
        </div>
      ) : (
        resources.data?.configured && (
          <>
            <div className="flex flex-wrap gap-2">
              {(["publish_images", "restart_pods"] as const).map((value) => (
                <Button
                  key={value}
                  variant={action === value ? "brandSubtle" : "outline"}
                  aria-pressed={action === value}
                  onClick={() => setAction(value)}
                >
                  {value === "publish_images"
                    ? t(($) => $.infrastructure.image_release.publish)
                    : t(($) => $.infrastructure.image_release.restart)}
                </Button>
              ))}
            </div>
            <p className="text-caption text-muted-foreground">
              {t(($) =>
                action === "restart_pods"
                  ? $.infrastructure.image_release.restart_hint
                  : $.infrastructure.image_release.scope_hint,
              )}
            </p>
            {requestedRole &&
              !resources.data.targets.some(
                (target) => target.role_type === requestedRole,
              ) && (
                <p className="text-caption text-warning">
                  {t(($) => $.infrastructure.image_release.unmatched_role)}
                </p>
              )}
            <form
              className="grid gap-4"
              onSubmit={(event) => {
                event.preventDefault();
                preview.mutate();
              }}
            >
              {resources.data.targets.map((target) => (
                <div
                  key={target.key}
                  className="grid min-w-0 gap-3 rounded-lg border p-4"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <label className="flex min-w-0 items-center gap-2 text-body font-medium">
                      <input
                        type="checkbox"
                        checked={
                          selected[target.key] === true &&
                          !target.blocked_reason
                        }
                        disabled={
                          !canManage ||
                          preview.isPending ||
                          !!target.blocked_reason
                        }
                        aria-label={t(
                          ($) => $.infrastructure.image_release.select_role,
                          {
                            role: `${target.role_name || target.container} · ${target.workload} · ${target.container}`,
                          },
                        )}
                        onChange={(event) =>
                          setSelected({
                            ...selected,
                            [target.key]: event.target.checked,
                          })
                        }
                      />
                      {target.role_name || target.container}
                    </label>
                    <span className="text-caption text-muted-foreground">
                      {t(($) => $.infrastructure.image_release.pods_count, {
                        count: target.replicas,
                      })}
                    </span>
                  </div>
                  <p className="break-all text-caption text-muted-foreground">
                    {target.namespace} / {target.kind} / {target.workload} /{" "}
                    {target.container}
                  </p>
                  {target.blocked_reason && (
                    <p className="text-caption text-warning">
                      {target.blocked_reason}
                    </p>
                  )}
                  <div className="grid gap-1">
                    <p className="text-caption text-muted-foreground">
                      {t(($) => $.infrastructure.image_release.current_image)}
                    </p>
                    <code className="break-all text-caption">
                      {target.image}
                    </code>
                  </div>
                  {action === "publish_images" && (
                    <Field
                      label={t(($) => $.infrastructure.image_release.source, {
                        role: `${target.role_name || target.container} · ${target.workload}`,
                      })}
                    >
                      <Input
                        value={sources[target.key] ?? ""}
                        required={
                          selected[target.key] === true &&
                          !target.blocked_reason
                        }
                        disabled={
                          !canManage ||
                          !selected[target.key] ||
                          preview.isPending ||
                          !!target.blocked_reason
                        }
                        spellCheck={false}
                        autoComplete="off"
                        placeholder={t(
                          ($) =>
                            $.infrastructure.image_release.source_placeholder,
                        )}
                        onChange={(event) =>
                          setSources({
                            ...sources,
                            [target.key]: event.target.value,
                          })
                        }
                      />
                    </Field>
                  )}
                </div>
              ))}
              {!resources.data.targets.length && (
                <p className="text-body text-muted-foreground">
                  {t(($) => $.infrastructure.image_release.empty)}
                </p>
              )}
              {canManage && resources.data.targets.length > 0 && (
                <>
                  <Field
                    label={t(($) => $.infrastructure.image_release.reason)}
                  >
                    <Textarea
                      rows={2}
                      maxLength={2000}
                      value={reason}
                      onChange={(event) => setReason(event.target.value)}
                    />
                  </Field>
                  <ErrorMessage error={preview.error} />
                  <div className="flex justify-end">
                    <Button
                      type="submit"
                      disabled={count === 0 || count > 16 || preview.isPending}
                      aria-busy={preview.isPending}
                    >
                      {t(($) => $.infrastructure.image_release.preview)} ·{" "}
                      {count}
                    </Button>
                  </div>
                </>
              )}
            </form>
          </>
        )
      )}
    </section>
  );
}
