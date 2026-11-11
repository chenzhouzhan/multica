"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import {
  infrastructureResourceKey,
  metainfoDraftKey,
  setMetainfoDraft,
  useMetainfoDraftStore,
  useInfrastructureEditorStore,
  requestInfrastructureNavigation,
  type InfrastructureConnection,
  type InfrastructureService,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { CodeEditor } from "@multica/ui/components/ui/code-editor";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@multica/ui/components/ui/dialog";
import { FileCode, FileEdit, Save } from "@multica/ui/icons";
import { useT } from "../i18n";
import { useNavigation } from "../navigation";
import { ErrorMessage } from "./components";
import { MetainfoFileTree } from "./metainfo-file-tree";

export function ServiceMetainfo({
  ws,
  connection,
  service,
  canManage,
}: {
  ws: string;
  connection: InfrastructureConnection;
  service: InfrastructureService;
  canManage: boolean;
}) {
  const { t } = useT("runtimes");
  const nav = useNavigation();
  const q = useQueryClient();
  const [selected, setSelected] = useState("");
  const [editingFile, setEditingFile] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const [discard, setDiscard] = useState(false);
  const pending = useRef<(() => void) | null>(null);
  const key = [
    ...infrastructureResourceKey(ws, connection.id),
    "service-meta",
    service.id,
    service.type,
    service.version,
  ];
  const files = useQuery({
    queryKey: [...key, "files"],
    queryFn: () => api.serviceMetaFiles(ws, connection.id, service.id),
  });
  const entry =
    files.data?.find((file) => file.key === selected) ??
    files.data?.find((file) => file.type === "INFO_YAML") ??
    files.data?.[0];
  const fileKey = [...key, "file", entry?.key];
  const file = useQuery({
    queryKey: fileKey,
    queryFn: () =>
      api.serviceMetaFile(ws, connection.id, service.id, entry!.key),
    enabled: !!entry,
    refetchOnWindowFocus: false,
  });
  const draftKey = metainfoDraftKey(
    ws,
    connection.id,
    service.type,
    service.version,
    entry?.key ?? "",
  );
  const draft = useMetainfoDraftStore(
    (state) => state.draft.files[draftKey] ?? null,
  );
  const dirty = draft !== null && draft !== file.data?.content;
  const editing = editingFile === entry?.key || draft !== null;
  const save = useMutation({
    retry: false,
    mutationFn: () =>
      api.saveServiceMetaFile(
        ws,
        connection.id,
        service.id,
        entry!.key,
        draft ?? file.data!.content,
      ),
    onSuccess: () => {
      void q.invalidateQueries({
        queryKey: [
          ...infrastructureResourceKey(ws, connection.id),
          "service-meta",
        ],
        refetchType: "none",
      });
      q.setQueryData(fileKey, {
        key: entry!.key,
        text: true,
        content: draft ?? file.data!.content,
      });
      setMetainfoDraft(draftKey, null);
      setEditingFile(null);
      setSaved(true);
    },
  });
  const reset = () => {
    setMetainfoDraft(draftKey, null);
    setEditingFile(null);
    setSaved(false);
    save.reset();
  };

  useEffect(() => {
    if (!dirty && !save.isPending) return;
    const guard = (proceed: () => void) => {
      if (save.isPending) return;
      pending.current = proceed;
      setDiscard(true);
    };
    useInfrastructureEditorStore.getState().setGuard(guard);
    const beforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    const captureLink = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey ||
        !(event.target instanceof Element)
      )
        return;
      const anchor = event.target.closest<HTMLAnchorElement>("a[href]");
      if (
        !anchor ||
        anchor.target === "_blank" ||
        anchor.hasAttribute("download")
      )
        return;
      const target = new URL(anchor.href, window.location.href);
      const appOrigin = new URL(nav.getShareableUrl("/"), window.location.href)
        .origin;
      if (target.origin !== appOrigin) return;
      const path = target.pathname + target.search + target.hash;
      if (
        path ===
        nav.pathname +
          (nav.searchParams.size ? `?${nav.searchParams}` : "") +
          nav.hash
      )
        return;
      event.preventDefault();
      event.stopImmediatePropagation();
      guard(() => nav.push(path));
    };
    window.addEventListener("beforeunload", beforeUnload);
    document.addEventListener("click", captureLink, true);
    return () => {
      if (useInfrastructureEditorStore.getState().guard === guard)
        useInfrastructureEditorStore.getState().setGuard(null);
      window.removeEventListener("beforeunload", beforeUnload);
      document.removeEventListener("click", captureLink, true);
    };
  }, [dirty, save.isPending, nav]);

  const language =
    entry && /\.ya?ml$/i.test(entry.name)
      ? "yaml"
      : entry && /\.properties$/i.test(entry.name)
        ? "properties"
        : "text";
  return (
    <section className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex items-start gap-2.5 rounded-lg border bg-muted/40 p-3 text-caption text-muted-foreground">
        <FileCode className="mt-0.5 size-4 shrink-0" />
        <div>
          <p className="break-words font-medium text-foreground">
            {connection.name} / {service.type} / {service.version}
          </p>
          <p className="mt-1">{t(($) => $.infrastructure.metainfo_scope)}</p>
        </div>
      </div>
      <ErrorMessage error={files.error} />
      {files.isPending && (
        <p role="status">{t(($) => $.infrastructure.loading)}</p>
      )}
      {files.data?.length === 0 && (
        <p>{t(($) => $.infrastructure.no_results)}</p>
      )}
      {!!files.data?.length && (
        <div className="grid min-h-96 min-w-0 overflow-hidden rounded-lg border md:grid-cols-[minmax(11rem,1fr)_minmax(0,4fr)]">
          <nav
            aria-label={t(($) => $.infrastructure.metainfo_files)}
            className="max-h-40 overflow-y-auto border-b bg-sidebar p-2 md:max-h-[65dvh] md:border-b-0 md:border-r"
          >
            <p className="px-2 pb-3 pt-1 text-caption text-muted-foreground">
              {t(($) => $.infrastructure.metainfo_files)}
            </p>
            <MetainfoFileTree
              files={files.data}
              selectedKey={entry?.key ?? ""}
              disabled={save.isPending}
              onSelect={(item) => {
                if (item.key !== entry?.key)
                  requestInfrastructureNavigation(() => {
                    reset();
                    setSelected(item.key);
                  });
              }}
            />
          </nav>
          <div className="flex min-w-0 flex-col">
            <div className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
              <p className="min-w-0 break-all font-mono text-caption">
                {entry?.name}
              </p>
              <div className="flex items-center gap-2">
                {canManage &&
                  file.data?.text &&
                  (editing ? (
                    <>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={save.isPending}
                        onClick={() => requestInfrastructureNavigation(reset)}
                      >
                        {t(($) => $.infrastructure.cancel)}
                      </Button>
                      <Button
                        size="sm"
                        disabled={!dirty || save.isPending}
                        aria-busy={save.isPending}
                        onClick={() => save.mutate()}
                      >
                        <Save />
                        {t(($) =>
                          save.isPending
                            ? $.infrastructure.metainfo_saving
                            : $.infrastructure.metainfo_save,
                        )}
                      </Button>
                    </>
                  ) : (
                    <Button
                      size="sm"
                      onClick={() => {
                        setEditingFile(entry!.key);
                        setSaved(false);
                      }}
                    >
                      <FileEdit />
                      {t(($) => $.infrastructure.metainfo_edit)}
                    </Button>
                  ))}
              </div>
            </div>
            <div className="grid gap-2 empty:hidden">
              <ErrorMessage error={file.error} />
              <ErrorMessage error={save.error} />
            </div>
            {file.isPending && (
              <p role="status" className="p-4">
                {t(($) => $.infrastructure.loading)}
              </p>
            )}
            {file.data &&
              (file.data.text ? (
                <CodeEditor
                  key={entry!.key}
                  aria-label={t(($) => $.infrastructure.metainfo_content)}
                  className="h-[max(24rem,calc(100dvh-25rem))] min-h-96"
                  language={language}
                  readOnly={!canManage || !editing || save.isPending}
                  value={draft ?? file.data.content}
                  onChange={(value) => {
                    setMetainfoDraft(
                      draftKey,
                      value === file.data.content ? null : value,
                    );
                    setSaved(false);
                  }}
                />
              ) : (
                <p className="p-4">
                  {t(($) => $.infrastructure.metainfo_binary)}
                </p>
              ))}
            <div className="flex flex-wrap items-center justify-between gap-2 border-t px-3 py-2 text-caption text-muted-foreground">
              <span>{`${language.toUpperCase()} · UTF-8`}</span>
              {saved ? (
                <span role="status" className="text-success">
                  {t(($) => $.infrastructure.metainfo_saved)}
                </span>
              ) : (
                <span>
                  {t(($) =>
                    dirty
                      ? $.infrastructure.metainfo_unsaved
                      : !canManage
                        ? $.infrastructure.metainfo_readonly
                        : $.infrastructure.metainfo_read_mode,
                  )}
                </span>
              )}
            </div>
          </div>
        </div>
      )}
      <Dialog
        open={discard}
        onOpenChange={(value) => {
          if (!value) pending.current = null;
          setDiscard(value);
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t(($) => $.infrastructure.discard_metainfo)}
            </DialogTitle>
            <DialogDescription>{entry?.name}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                pending.current = null;
                setDiscard(false);
              }}
            >
              {t(($) => $.infrastructure.keep_editing)}
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                const proceed = pending.current;
                pending.current = null;
                reset();
                useInfrastructureEditorStore.getState().setGuard(null);
                setDiscard(false);
                proceed?.();
              }}
            >
              {t(($) => $.infrastructure.discard_changes)}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </section>
  );
}
