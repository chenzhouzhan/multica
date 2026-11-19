"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import {
  infrastructureResourceKey,
  type InfrastructureExecutionAccess,
  type InfrastructureExecutionAccessInput,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@multica/ui/components/ui/dialog";
import { useT } from "../i18n";
import { ErrorMessage, Field, Picker } from "./components";

const emptyInput = (): InfrastructureExecutionAccessInput => ({
  host: "",
  port: 22,
  username: "",
  host_key: "",
  context: "",
  auth_type: "password",
  private_key: "",
  password: "",
});

export function ExecutionAccessDialog({
  ws,
  connection,
  cluster,
}: {
  ws: string;
  connection: string;
  cluster: number;
}) {
  const { t } = useT("runtimes");
  const q = useQueryClient();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState(emptyInput);
  const [contexts, setContexts] = useState<
    InfrastructureExecutionAccess["context_options"]
  >([]);
  const changeIdentity = (
    change: Partial<InfrastructureExecutionAccessInput>,
  ) => {
    setInput({
      ...input,
      ...change,
      context: "",
      host_key:
        change.host !== undefined || change.port !== undefined
          ? ""
          : input.host_key,
    });
    setContexts([]);
    save.reset();
  };
  const saved = useQuery({
    queryKey: [
      ...infrastructureResourceKey(ws, connection),
      cluster,
      "execution-access",
    ],
    queryFn: () => api.infrastructureExecutionAccess(ws, connection, cluster),
    enabled: open,
    refetchOnWindowFocus: false,
  });
  useEffect(() => {
    if (open && saved.data) {
      const { host, port, username, auth_type } = saved.data;
      setInput({ ...emptyInput(), host, port, username, auth_type });
      setContexts([]);
    }
  }, [open, saved.data]);
  const save = useMutation({
    retry: false,
    mutationFn: (value: InfrastructureExecutionAccessInput) =>
      api.saveInfrastructureExecutionAccess(ws, connection, cluster, value),
    onSuccess: async (result) => {
      if (result.requires_host_key_confirmation) {
        setContexts([]);
        setInput((previous) => ({ ...previous, context: "" }));
        return;
      }
      if (result.requires_context_selection) {
        setContexts(result.context_options);
        setInput((previous) => ({
          ...previous,
          context: "",
          host_key: result.host_key,
        }));
        return;
      }
      q.setQueryData(
        [
          ...infrastructureResourceKey(ws, connection),
          cluster,
          "execution-access",
        ],
        result,
      );
      await q.invalidateQueries({
        queryKey: infrastructureResourceKey(ws, connection),
      });
      setInput(emptyInput());
      setOpen(false);
    },
  });
  const review =
    save.data?.requires_host_key_confirmation &&
    save.data.host === input.host.trim() &&
    save.data.port === input.port
      ? save.data
      : null;
  const knownHost =
    saved.data?.configured &&
    saved.data.host === input.host.trim() &&
    saved.data.port === input.port;
  const retainCredentials =
    knownHost &&
    saved.data?.username === input.username.trim() &&
    saved.data.auth_type === input.auth_type;
  const fingerprint =
    review?.host_key_fingerprint ||
    (knownHost ? saved.data?.host_key_fingerprint : "");
  const hostKeyChanged =
    !!review?.previous_host_key_fingerprint &&
    review.previous_host_key_fingerprint !== review.host_key_fingerprint;
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!save.isPending) {
          setOpen(value);
          if (!value) {
            setInput(emptyInput());
            setContexts([]);
            save.reset();
          }
        }
      }}
    >
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        {t(($) => $.infrastructure.image_release.settings)}
      </DialogTrigger>
      <DialogContent className="max-h-[85dvh] overflow-y-auto">
        <form
          className="grid gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            const value = {
              ...input,
              host_key: review?.host_key || input.host_key,
              probe_host_key: false,
            };
            setInput(value);
            save.mutate(value);
          }}
        >
          <DialogHeader>
            <DialogTitle>
              {t(($) => $.infrastructure.image_release.settings)}
            </DialogTitle>
            <DialogDescription>
              {t(($) => $.infrastructure.image_release.access_hint)}
            </DialogDescription>
          </DialogHeader>
          <ErrorMessage error={saved.error} />
          <fieldset
            disabled={saved.isPending || save.isPending}
            className="grid min-w-0 gap-4 border-0 p-0"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t(($) => $.infrastructure.image_release.host)}>
                <Input
                  required
                  value={input.host}
                  onChange={(event) =>
                    changeIdentity({ host: event.target.value })
                  }
                  autoComplete="off"
                />
              </Field>
              <Field label={t(($) => $.infrastructure.image_release.port)}>
                <Input
                  required
                  type="number"
                  min={1}
                  max={65535}
                  value={input.port}
                  onChange={(event) =>
                    changeIdentity({ port: Number(event.target.value) })
                  }
                />
              </Field>
              <Field label={t(($) => $.infrastructure.image_release.ssh_user)}>
                <Input
                  required
                  value={input.username}
                  onChange={(event) =>
                    changeIdentity({ username: event.target.value })
                  }
                  autoComplete="off"
                />
              </Field>
            </div>
            {contexts.length > 1 && (
              <div className="grid gap-2">
                <Picker
                  label={t(($) => $.infrastructure.image_release.context)}
                  value={input.context}
                  items={contexts.map((context) => ({
                    value: context.name,
                    label:
                      context.cluster && context.cluster !== context.name
                        ? `${context.name} · ${context.cluster}`
                        : context.name,
                  }))}
                  onChange={(context) => setInput({ ...input, context })}
                />
                <p className="text-caption text-muted-foreground">
                  {t(($) => $.infrastructure.image_release.context_hint)}
                </p>
              </div>
            )}
            <Picker
              label={t(($) => $.infrastructure.image_release.auth_type)}
              value={input.auth_type}
              items={[
                {
                  value: "key",
                  label: t(($) => $.infrastructure.image_release.private_key),
                },
                {
                  value: "password",
                  label: t(($) => $.infrastructure.image_release.password),
                },
              ]}
              onChange={(value) =>
                changeIdentity({
                  auth_type: value === "password" ? "password" : "key",
                  private_key: "",
                  password: "",
                })
              }
            />
            {input.auth_type === "key" ? (
              <Field
                label={t(($) => $.infrastructure.image_release.private_key)}
              >
                <Textarea
                  required={!retainCredentials}
                  rows={5}
                  value={input.private_key}
                  onChange={(event) =>
                    setInput({ ...input, private_key: event.target.value })
                  }
                  spellCheck={false}
                  autoComplete="off"
                />
              </Field>
            ) : (
              <Field label={t(($) => $.infrastructure.image_release.password)}>
                <Input
                  type="password"
                  required={!retainCredentials}
                  value={input.password}
                  onChange={(event) =>
                    setInput({ ...input, password: event.target.value })
                  }
                  autoComplete="new-password"
                />
              </Field>
            )}
            {fingerprint && (
              <section
                aria-label={t(($) => $.infrastructure.image_release.host_key)}
                className="grid min-w-0 gap-2 rounded-lg border p-3"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-caption text-muted-foreground">
                    {t(($) => $.infrastructure.image_release.host_key)}
                  </p>
                  {knownHost && !review && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="xs"
                      onClick={() => {
                        setContexts([]);
                        save.mutate({
                          ...input,
                          host_key: "",
                          probe_host_key: true,
                        });
                      }}
                    >
                      {t(
                        ($) => $.infrastructure.image_release.host_key_recheck,
                      )}
                    </Button>
                  )}
                </div>
                {hostKeyChanged && (
                  <div className="grid gap-1">
                    <p className="text-caption text-muted-foreground">
                      {t(
                        ($) => $.infrastructure.image_release.host_key_previous,
                      )}
                    </p>
                    <code className="break-all text-caption text-muted-foreground">
                      {review.previous_host_key_fingerprint}
                    </code>
                  </div>
                )}
                <code className="break-all text-caption">{fingerprint}</code>
                {review && (
                  <p
                    className={
                      hostKeyChanged
                        ? "text-caption text-warning"
                        : "text-caption text-muted-foreground"
                    }
                  >
                    {t(($) =>
                      hostKeyChanged
                        ? $.infrastructure.image_release.host_key_changed
                        : $.infrastructure.image_release.host_key_hint,
                    )}
                  </p>
                )}
              </section>
            )}
            {retainCredentials && (
              <p className="text-caption text-muted-foreground">
                {t(($) => $.infrastructure.image_release.secret_hint)}
              </p>
            )}
          </fieldset>
          <ErrorMessage error={save.error} />
          <DialogFooter>
            <DialogClose
              render={<Button variant="outline" disabled={save.isPending} />}
            >
              {t(($) => $.infrastructure.cancel)}
            </DialogClose>
            <Button
              type="submit"
              disabled={
                saved.isPending ||
                save.isPending ||
                saved.isError ||
                (contexts.length > 1 && !input.context)
              }
              aria-busy={save.isPending}
            >
              {t(($) =>
                review
                  ? $.infrastructure.image_release.host_key_trust
                  : $.infrastructure.image_release.save_access,
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
