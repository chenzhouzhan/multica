"use client";

import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { Plus, Trash2 } from "@multica/ui/icons";
import {
  weeklyRowId,
  type WeeklyTask,
  type WeeklyOverview,
  type WeeklyProgress,
  type WeeklyCollection,
} from "@multica/core/weekly-reports";
import { useT } from "../i18n";

export const taskFields = [
  { key: "version", label: "version" },
  { key: "title", label: "title" },
  { key: "completed", label: "completed" },
  { key: "spent", label: "spent" },
  { key: "remaining", label: "remaining" },
  { key: "note", label: "note" },
] as const;
export const progressFields = [
  { key: "title", label: "progressTitle" },
  { key: "type", label: "type" },
  { key: "lastProgress", label: "lastProgress" },
  { key: "thisPlan", label: "thisPlan" },
  { key: "note", label: "note" },
] as const;

export function TaskTable({
  tasks,
  onChange,
  collection,
}: {
  tasks: WeeklyTask[];
  onChange?: (tasks: WeeklyTask[]) => void;
  collection?: WeeklyCollection;
}) {
  const { t } = useT("weekly-reports");
  const entries = collection
    ? collection.reports.flatMap(({ member, content }) =>
        content.tasks.map((task) => ({ task, member })),
      )
    : tasks.map((task) => ({ task, member: null }));
  return (
    <>
      <div
        className="w-table-region"
        role="region"
        aria-label={t(($) => $.page.progress)}
        tabIndex={0}
      >
        <table
          className={`w-table w-task-table ${collection ? "w-summary-table" : ""}`}
          role="table"
        >
          <thead role="rowgroup">
            <tr role="row">
              {collection && (
                <th scope="col" role="columnheader">
                  {t(($) => $.fields.member)}
                </th>
              )}
              {taskFields.map((field) => (
                <th
                  scope="col"
                  role="columnheader"
                  key={field.key}
                  data-field={field.key}
                >
                  {t(($) => $.fields[field.label])}
                </th>
              ))}
              {onChange && (
                <th scope="col" role="columnheader">
                  <span className="sr-only">{t(($) => $.actions.edit)}</span>
                </th>
              )}
            </tr>
          </thead>
          <tbody role="rowgroup">
            {entries.map(({ task, member }, index) => (
              <tr role="row" key={`${member?.userId ?? ""}:${task.id}`}>
                {member && (
                  <td role="cell" data-field="member">
                    {member.name}
                  </td>
                )}
                {taskFields.map((field) => {
                  const numeric =
                    field.key === "spent" || field.key === "remaining";
                  const label = t(($) => $.fields[field.label]);
                  const change = (value: string) =>
                    onChange?.(
                      tasks.map((row) =>
                        row.id === task.id
                          ? {
                              ...row,
                              [field.key]: numeric
                                ? value === ""
                                  ? null
                                  : Number(value)
                                : value,
                            }
                          : row,
                      ),
                    );
                  return (
                    <td
                      role="cell"
                      key={field.key}
                      data-field={field.key}
                      data-label={label}
                      data-empty={
                        task[field.key] == null || task[field.key] === ""
                      }
                    >
                      {onChange ? (
                        numeric || field.key === "version" ? (
                          <Input
                            aria-label={`${index + 1}: ${label}`}
                            type={numeric ? "number" : "text"}
                            min={numeric ? 0 : undefined}
                            step={numeric ? "any" : undefined}
                            value={task[field.key] ?? ""}
                            onChange={(event) => change(event.target.value)}
                          />
                        ) : (
                          <Textarea
                            rows={2}
                            aria-label={`${index + 1}: ${label}`}
                            value={String(task[field.key] ?? "")}
                            onChange={(event) => change(event.target.value)}
                          />
                        )
                      ) : (
                        <span
                          className={
                            numeric
                              ? "w-number"
                              : field.key === "title"
                                ? "w-task-title"
                                : ""
                          }
                        >
                          {task[field.key] ?? t(($) => $.states.unknown)}
                        </span>
                      )}
                    </td>
                  );
                })}
                {onChange && (
                  <td className="w-row-actions" role="cell">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t(($) => $.actions.remove_task, {
                        index: index + 1,
                      })}
                      onClick={() =>
                        onChange(tasks.filter((row) => row.id !== task.id))
                      }
                    >
                      <Trash2 />
                    </Button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {entries.length === 0 && (
          <p className="w-empty-row">{t(($) => $.page.empty_progress)}</p>
        )}
      </div>
      {onChange && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="w-add"
          onClick={() =>
            onChange([
              ...tasks,
              {
                id: weeklyRowId(),
                version: "",
                title: "",
                completed: "",
                spent: null,
                remaining: null,
                note: "",
              },
            ])
          }
        >
          <Plus />
          {t(($) => $.actions.add_task)}
        </Button>
      )}
      <WorkloadTotals tasks={entries.map(({ task }) => task)} />
    </>
  );
}

function WorkloadTotals({ tasks }: { tasks: WeeklyTask[] }) {
  const { t } = useT("weekly-reports");
  return (
    <div className="w-totals">
      {(["spent", "remaining"] as const).map((key) => {
        const known = tasks.filter((task) => task[key] !== null);
        return (
          <span key={key}>
            {t(($) =>
              key === "spent" ? $.fields.totalSpent : $.fields.totalRemaining,
            )}
            <strong>
              {known.length
                ? Math.round(
                    known.reduce((sum, row) => sum + (row[key] ?? 0), 0) * 100,
                  ) / 100
                : t(($) => $.states.unknown)}
            </strong>
            <small>
              {known.length} / {tasks.length}
            </small>
          </span>
        );
      })}
    </div>
  );
}

export function ProgressTables({
  overview,
  onChange,
}: {
  overview: WeeklyOverview;
  onChange?: (value: WeeklyOverview) => void;
}) {
  const { t } = useT("weekly-reports");
  const groups = [
    { key: "versionRows", label: "version_progress" },
    { key: "onsiteRows", label: "onsite" },
  ] as const;
  return (
    <div className="w-progress-groups">
      {groups.map((group) => {
        const rows = overview[group.key];
        const update = (next: WeeklyProgress[]) =>
          onChange?.({ ...overview, [group.key]: next });
        return (
          <section
            id={`weekly-${group.key}`}
            className="w-progress-group"
            key={group.key}
          >
            <h4>
              {group.key === "versionRows" && overview.versionName
                ? `${overview.versionName} `
                : ""}
              {t(($) => $.page[group.label])}
              <span className="w-count">{rows.length}</span>
            </h4>
            {onChange && group.key === "versionRows" && (
              <label className="w-version-label">
                {t(($) => $.fields.versionName)}
                <Input
                  value={overview.versionName}
                  onChange={(event) =>
                    onChange({ ...overview, versionName: event.target.value })
                  }
                />
              </label>
            )}
            <div
              className="w-table-region"
              role="region"
              tabIndex={0}
              aria-label={t(($) => $.page[group.label])}
            >
              <table className="w-table w-progress-table" role="table">
                <thead role="rowgroup">
                  <tr role="row">
                    {progressFields.map((field) => (
                      <th
                        scope="col"
                        role="columnheader"
                        data-field={field.key}
                        key={field.key}
                      >
                        {t(($) => $.fields[field.label])}
                      </th>
                    ))}
                    {onChange && (
                      <th scope="col" role="columnheader">
                        <span className="sr-only">
                          {t(($) => $.actions.edit)}
                        </span>
                      </th>
                    )}
                  </tr>
                </thead>
                <tbody role="rowgroup">
                  {rows.map((row, index) => (
                    <tr role="row" key={row.id}>
                      {progressFields.map((field) => (
                        <td
                          role="cell"
                          key={field.key}
                          data-field={field.key}
                          data-label={t(($) => $.fields[field.label])}
                          data-empty={!row[field.key]}
                        >
                          {onChange ? (
                            field.key === "type" ? (
                              <Input
                                aria-label={`${t(($) => $.page[group.label])} ${index + 1}: ${t(($) => $.fields[field.label])}`}
                                value={row[field.key]}
                                onChange={(event) =>
                                  update(
                                    rows.map((current) =>
                                      current.id === row.id
                                        ? {
                                            ...current,
                                            [field.key]: event.target.value,
                                          }
                                        : current,
                                    ),
                                  )
                                }
                              />
                            ) : (
                              <Textarea
                                rows={2}
                                aria-label={`${t(($) => $.page[group.label])} ${index + 1}: ${t(($) => $.fields[field.label])}`}
                                value={row[field.key]}
                                onChange={(event) =>
                                  update(
                                    rows.map((current) =>
                                      current.id === row.id
                                        ? {
                                            ...current,
                                            [field.key]: event.target.value,
                                          }
                                        : current,
                                    ),
                                  )
                                }
                              />
                            )
                          ) : (
                            <span>{row[field.key] || "—"}</span>
                          )}
                        </td>
                      ))}
                      {onChange && (
                        <td role="cell" className="w-row-actions">
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon-sm"
                            aria-label={t(($) => $.actions.remove_progress, {
                              index: index + 1,
                            })}
                            onClick={() =>
                              update(
                                rows.filter((current) => current.id !== row.id),
                              )
                            }
                          >
                            <Trash2 />
                          </Button>
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
              {rows.length === 0 && (
                <p className="w-empty-row">{t(($) => $.page.empty_progress)}</p>
              )}
            </div>
            {onChange && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-add"
                onClick={() =>
                  update([
                    ...rows,
                    {
                      id: weeklyRowId(),
                      title: "",
                      type: "",
                      lastProgress: "",
                      thisPlan: "",
                      note: "",
                    },
                  ])
                }
              >
                <Plus />
                {t(($) =>
                  group.key === "versionRows"
                    ? $.actions.add_version
                    : $.actions.add_onsite,
                )}
              </Button>
            )}
          </section>
        );
      })}
    </div>
  );
}
