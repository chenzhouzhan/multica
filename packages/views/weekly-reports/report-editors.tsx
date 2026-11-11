"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@multica/core/api";
import {
  emptyWeeklyContent,
  emptyWeeklyOverview,
  weekRange,
  useReportDraft,
  useSaveWeeklyReport,
  useSubmitWeeklyReport,
  useSaveWeeklyTeam,
  useGenerateWeeklyTeam,
  type WeeklyReport,
  type WeeklyTeam,
  type WeeklyContent,
  type WeeklyOverview,
  type DraftStatus,
} from "@multica/core/weekly-reports";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { ArrowLeft, Check, Download, Lock } from "@multica/ui/icons";
import { useT } from "../i18n";
import { ProgressTables, TaskTable } from "./report-tables";
import { downloadWeeklyFile } from "./report-export";

export type RegisterSave = (save: (() => Promise<number>) | null) => void;
function useEditorGuard(
  status: DraftStatus,
  flush: () => Promise<number>,
  register: RegisterSave,
) {
  useEffect(() => {
    register(flush);
    return () => register(null);
  }, [flush, register]);
  useEffect(() => {
    if (status === "saved") return;
    const prevent = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", prevent);
    return () => window.removeEventListener("beforeunload", prevent);
  }, [status]);
}

function SaveFeedback({
  status,
  error,
  content,
  retry,
  reload,
}: {
  status: DraftStatus;
  error: unknown;
  content: unknown;
  retry: () => Promise<number>;
  reload: () => void;
}) {
  const { t } = useT("weekly-reports");
  if (!error) return null;
  const conflict = error instanceof ApiError && error.status === 409;
  return (
    <div className="w-error" role="alert">
      <p>{t(($) => (conflict ? $.errors.conflict : $.errors.save))}</p>
      <div className="w-actions">
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            downloadWeeklyFile(
              "weekly-report-draft.json",
              JSON.stringify(content, null, 2),
              "application/json",
            )
          }
        >
          <Download />
          {t(($) => $.actions.download_draft)}
        </Button>
        {status === "error" && !conflict && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void retry().catch(() => undefined);
            }}
          >
            {t(($) => $.actions.retry)}
          </Button>
        )}
        <Button variant="outline" size="sm" onClick={reload}>
          {t(($) => $.actions.reload)}
        </Button>
      </div>
    </div>
  );
}

export function DraftState({ status }: { status: DraftStatus }) {
  const { t } = useT("weekly-reports");
  return (
    <span className="w-save-state" role="status">
      {t(($) =>
        status === "saved"
          ? $.states.saved
          : status === "saving"
            ? $.states.saving
            : status === "unsaved"
              ? $.states.unsaved
              : $.states.save_failed,
      )}
    </span>
  );
}

export function PersonalEditor({
  wsId,
  week,
  report,
  register,
  close,
  reload,
}: {
  wsId: string;
  week: string;
  report: WeeklyReport;
  register: RegisterSave;
  close: () => void;
  reload: () => void;
}) {
  const { t } = useT("weekly-reports");
  const save = useSaveWeeklyReport(wsId, week);
  const submit = useSubmitWeeklyReport(wsId, week);
  const saveAsync = save.mutateAsync;
  const persist = useCallback(
    async (content: WeeklyContent, revision: number) =>
      (await saveAsync({ content, revision })).revision,
    [saveAsync],
  );
  const draft = useReportDraft(
    report.draft ?? emptyWeeklyContent(),
    report.revision,
    persist,
  );
  const [preview, setPreview] = useState(false);
  const [validation, setValidation] = useState("");
  useEditorGuard(draft.status, draft.flush, register);
  async function publish() {
    const content = draft.content;
    if (!content.tasks.length) {
      setValidation(t(($) => $.errors.tasks));
      return;
    }
    if (
      content.tasks.some((task) => !task.title.trim() || !task.completed.trim())
    ) {
      setValidation(t(($) => $.errors.task_fields));
      return;
    }
    if (
      content.tasks.some((task) =>
        [task.spent, task.remaining].some(
          (value) => value !== null && (!Number.isFinite(value) || value < 0),
        ),
      )
    ) {
      setValidation(t(($) => $.errors.workload));
      return;
    }
    setValidation("");
    try {
      const revision = await draft.flush();
      await submit.mutateAsync(revision);
      close();
    } catch {
      /* The editor preserves input and exposes the failing mutation. */
    }
  }
  return (
    <section className="w-reader w-editor">
      <div className="w-document-toolbar">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            void draft
              .flush()
              .then(close)
              .catch(() => undefined);
          }}
        >
          <ArrowLeft />
          {t(($) => $.actions.back)}
        </Button>
        <DraftState status={draft.status} />
        <Button
          variant="ghost"
          size="sm"
          className="w-push-right"
          aria-pressed={preview}
          onClick={() => setPreview(!preview)}
        >
          {t(($) => (preview ? $.actions.edit : $.actions.preview))}
        </Button>
      </div>
      <div className="w-document-scroll">
        <article className="w-document">
          <h2>{weekRange(week)}</h2>
          <SaveFeedback
            status={draft.status}
            error={draft.error ?? submit.error}
            content={draft.content}
            retry={draft.flush}
            reload={reload}
          />
          {validation && (
            <p className="w-error" role="alert">
              {validation}
            </p>
          )}
          <section className="w-section">
            <h3>
              {t(($) => $.page.progress)}
              <span className="w-count">{draft.content.tasks.length}</span>
            </h3>
            <p className="w-unit">{t(($) => $.page.unit)}</p>
            <TaskTable
              tasks={draft.content.tasks}
              onChange={
                preview
                  ? undefined
                  : (tasks) => {
                      setValidation("");
                      draft.change({ ...draft.content, tasks });
                    }
              }
            />
          </section>
          <section className="w-section">
            <h3>{t(($) => $.fields.plan)}</h3>
            {preview ? (
              <p className="w-prose">{draft.content.plan}</p>
            ) : (
              <Textarea
                aria-label={t(($) => $.fields.plan)}
                value={draft.content.plan}
                onChange={(event) =>
                  draft.change({ ...draft.content, plan: event.target.value })
                }
                rows={4}
              />
            )}
          </section>
        </article>
      </div>
      <footer className="w-editor-footer">
        <span>
          <Lock />
          {t(($) => $.page.submit_hint)}
        </span>
        <Button
          disabled={submit.isPending || draft.status === "saving"}
          onClick={() => {
            void publish();
          }}
        >
          <Check />
          {t(($) => (report.published ? $.actions.update : $.actions.submit))}
        </Button>
      </footer>
    </section>
  );
}

export function TeamEditor({
  wsId,
  week,
  report,
  register,
  close,
  reload,
}: {
  wsId: string;
  week: string;
  report: WeeklyTeam;
  register: RegisterSave;
  close: () => void;
  reload: () => void;
}) {
  const { t } = useT("weekly-reports");
  const save = useSaveWeeklyTeam(wsId, week);
  const generate = useGenerateWeeklyTeam(wsId, week);
  const saveAsync = save.mutateAsync;
  const persist = useCallback(
    async (overview: WeeklyOverview, revision: number) =>
      (await saveAsync({ overview, revision })).revision,
    [saveAsync],
  );
  const draft = useReportDraft(
    report.draft ?? emptyWeeklyOverview(),
    report.revision,
    persist,
  );
  const [preview, setPreview] = useState(false);
  const [validation, setValidation] = useState("");
  useEditorGuard(draft.status, draft.flush, register);
  async function publish() {
    const value = draft.content;
    if (!value.versionRows.length && !value.onsiteRows.length) {
      setValidation(t(($) => $.errors.progress));
      return;
    }
    if (value.versionRows.length && !value.versionName.trim()) {
      setValidation(t(($) => $.errors.version));
      return;
    }
    if (
      [...value.versionRows, ...value.onsiteRows].some(
        (row) => !row.title.trim(),
      )
    ) {
      setValidation(t(($) => $.errors.progress_title));
      return;
    }
    setValidation("");
    try {
      const revision = await draft.flush();
      await generate.mutateAsync(revision);
      close();
    } catch {
      /* Keep the overview available for correction or retry. */
    }
  }
  return (
    <section className="w-reader w-team w-editor">
      <div className="w-document-toolbar">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            void draft
              .flush()
              .then(close)
              .catch(() => undefined);
          }}
        >
          <ArrowLeft />
          {t(($) => $.actions.back)}
        </Button>
        <DraftState status={draft.status} />
        <Button
          variant="ghost"
          size="sm"
          className="w-push-right"
          aria-pressed={preview}
          onClick={() => setPreview(!preview)}
        >
          {t(($) => (preview ? $.actions.edit : $.actions.preview))}
        </Button>
      </div>
      <div className="w-document-scroll">
        <article className="w-document">
          <h2>{t(($) => $.page.leader)}</h2>
          <p className="w-meta">{weekRange(week)}</p>
          <SaveFeedback
            status={draft.status}
            error={draft.error ?? generate.error}
            content={draft.content}
            retry={draft.flush}
            reload={reload}
          />
          {validation && (
            <p className="w-error" role="alert">
              {validation}
            </p>
          )}
          <section className="w-section">
            <h3>{t(($) => $.page.overall)}</h3>
            <ProgressTables
              overview={draft.content}
              onChange={
                preview
                  ? undefined
                  : (value) => {
                      setValidation("");
                      draft.change(value);
                    }
              }
            />
          </section>
          <section className="w-section">
            <h3>{t(($) => $.page.summary)}</h3>
            <p className="w-unit">{t(($) => $.page.unit)}</p>
            <TaskTable tasks={[]} collection={report.collection} />
          </section>
        </article>
      </div>
      <footer className="w-editor-footer">
        <span>{t(($) => $.page.source_hint)}</span>
        <Button
          disabled={generate.isPending || draft.status === "saving"}
          onClick={() => {
            void publish();
          }}
        >
          <Check />
          {t(($) =>
            report.snapshot ? $.actions.regenerate : $.actions.generate,
          )}
        </Button>
      </footer>
    </section>
  );
}
