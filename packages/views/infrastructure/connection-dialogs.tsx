"use client";
import { useState, type FormEvent } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@multica/core/api";
import type {
  InfrastructureConnection,
  InfrastructureConnectionInput,
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
import { Plus } from "@multica/ui/icons";
import { useT } from "../i18n";
import { ErrorMessage, Field } from "./components";
export function ConnectionDialog({
  ws,
  connection,
  disabled,
  compact = false,
}: {
  ws: string;
  connection?: InfrastructureConnection;
  disabled?: boolean;
  compact?: boolean;
}) {
  const { t } = useT("runtimes");
  const q = useQueryClient();
  const [open, setOpen] = useState(false);
  const [input, setInput] = useState<InfrastructureConnectionInput>({
    name: connection?.name ?? "",
    url: connection?.url ?? "",
    username: connection?.username ?? "",
    password: "",
    ca_pem: connection?.ca_pem ?? "",
    skip_tls_verify: connection?.skip_tls_verify ?? false,
  });
  const save = useMutation({
    mutationFn: () =>
      api.saveInfrastructureConnection(ws, connection?.id ?? null, input),
    onSuccess: async () => {
      await q.invalidateQueries({ queryKey: ["infrastructure", ws] });
      setInput((value) => ({ ...value, password: "" }));
      setOpen(false);
    },
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate();
  };
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!save.isPending) {
          setOpen(value);
          if (!value) setInput((previous) => ({ ...previous, password: "" }));
        }
      }}
    >
      <DialogTrigger
        render={
          <Button
            variant={connection ? "outline" : "default"}
            disabled={disabled}
            size={compact ? "icon-sm" : "default"}
            aria-label={t(($) =>
              connection ? $.infrastructure.edit : $.infrastructure.add,
            )}
          />
        }
      >
        {!connection && <Plus />}
        <span className={compact ? "sr-only" : undefined}>
          {t(($) =>
            connection ? $.infrastructure.edit : $.infrastructure.add,
          )}
        </span>
      </DialogTrigger>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>
              {t(($) =>
                connection ? $.infrastructure.edit : $.infrastructure.add,
              )}
            </DialogTitle>
            <DialogDescription>
              {t(($) => $.infrastructure.credential_hint)}
            </DialogDescription>
          </DialogHeader>
          <Field label={t(($) => $.infrastructure.name)}>
            <Input
              required
              maxLength={120}
              value={input.name}
              onChange={(e) => setInput({ ...input, name: e.target.value })}
            />
          </Field>
          <Field label={t(($) => $.infrastructure.url)}>
            <Input
              required
              type="url"
              value={input.url}
              onChange={(e) => setInput({ ...input, url: e.target.value })}
            />
          </Field>
          <Field label={t(($) => $.infrastructure.username)}>
            <Input
              required
              autoComplete="off"
              value={input.username}
              onChange={(e) => setInput({ ...input, username: e.target.value })}
            />
          </Field>
          <Field label={t(($) => $.infrastructure.password)}>
            <Input
              type="password"
              required={!connection}
              autoComplete="new-password"
              value={input.password}
              onChange={(e) => setInput({ ...input, password: e.target.value })}
            />
          </Field>
          {connection && (
            <p className="text-caption text-muted-foreground">
              {t(($) => $.infrastructure.password_hint)}
            </p>
          )}
          <label className="flex items-center gap-2 text-body">
            <input
              type="checkbox"
              checked={input.skip_tls_verify}
              onChange={(e) =>
                setInput({ ...input, skip_tls_verify: e.target.checked })
              }
            />
            {t(($) => $.infrastructure.skip_tls_verify)}
          </label>
          {input.skip_tls_verify && (
            <p role="status" className="text-caption text-warning">
              {t(($) => $.infrastructure.skip_tls_verify_hint)}
            </p>
          )}
          <Field label={t(($) => $.infrastructure.ca)}>
            <Textarea
              rows={4}
              disabled={input.skip_tls_verify}
              value={input.ca_pem}
              onChange={(e) => setInput({ ...input, ca_pem: e.target.value })}
            />
          </Field>
          <p className="text-caption text-muted-foreground">
            {t(($) => $.infrastructure.ca_hint)}
          </p>
          <ErrorMessage error={save.error} />
          <DialogFooter>
            <DialogClose
              render={<Button variant="outline" disabled={save.isPending} />}
            >
              {t(($) => $.infrastructure.cancel)}
            </DialogClose>
            <Button
              type="submit"
              disabled={save.isPending}
              aria-busy={save.isPending}
            >
              {t(($) =>
                save.isPending
                  ? $.infrastructure.loading
                  : $.infrastructure.save,
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function RemoveConnection({
  ws,
  connection,
}: {
  ws: string;
  connection: InfrastructureConnection;
}) {
  const { t } = useT("runtimes");
  const q = useQueryClient();
  const [open, setOpen] = useState(false);
  const remove = useMutation({
    mutationFn: () => api.deleteInfrastructureConnection(ws, connection.id),
    onSuccess: async () => {
      await q.invalidateQueries({ queryKey: ["infrastructure", ws] });
      setOpen(false);
    },
  });
  return (
    <Dialog
      open={open}
      onOpenChange={(value) => {
        if (!remove.isPending) setOpen(value);
      }}
    >
      <DialogTrigger render={<Button variant="outline" />}>
        {t(($) => $.infrastructure.delete)}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t(($) => $.infrastructure.confirm_delete)}</DialogTitle>
          <DialogDescription>
            {connection.name} · {connection.url}
            <br />
            {t(($) => $.infrastructure.delete_hint)}
          </DialogDescription>
        </DialogHeader>
        <ErrorMessage error={remove.error} />
        <DialogFooter>
          <DialogClose
            render={<Button variant="outline" disabled={remove.isPending} />}
          >
            {t(($) => $.infrastructure.cancel)}
          </DialogClose>
          <Button
            variant="destructive"
            disabled={remove.isPending}
            onClick={() => remove.mutate()}
          >
            {t(($) => $.infrastructure.delete)}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
