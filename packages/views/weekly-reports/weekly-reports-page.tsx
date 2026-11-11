"use client";

import { useCallback, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useCurrentWorkspace, useWorkspacePaths } from "@multica/core/paths";
import {
  weeklyWeekOptions,
  weeklyMineOptions,
  weeklyHistoryOptions,
  weeklyTeamOptions,
  emptyWeeklyOverview,
  weekOf,
  shiftWeek,
  validWeek,
  weekRange,
  type WeeklyReport,
  type WeeklyTeam,
  type WeeklySnapshot,
} from "@multica/core/weekly-reports";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  ArrowLeft,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Download,
  FileText,
  Lock,
  PanelLeftClose,
  PanelLeftOpen,
  PencilLine,
  Search,
  Users,
} from "@multica/ui/icons";
import { useNavigation } from "../navigation";
import { useLocale, useT } from "../i18n";
import { PageHeader } from "../layout/page-header";
import {
  PersonalEditor,
  TeamEditor,
  type RegisterSave,
} from "./report-editors";
import {
  ProgressTables,
  TaskTable,
  taskFields,
  progressFields,
} from "./report-tables";
import { downloadWeeklyFile, weeklyReportHtml } from "./report-export";
import "./weekly-reports.css";

type View = "team" | "mine" | "leader";

function Status({ report }: { report?: WeeklyReport }) {
  const { t } = useT("weekly-reports");
  return (
    <span className={`w-status ${report?.published ? "submitted" : ""}`}>
      {t(($) => (report?.published ? $.states.submitted : $.states.pending))}
    </span>
  );
}

function TeamRead({
  report,
  week,
  canEdit,
  edit,
}: {
  report: WeeklyTeam;
  week: string;
  canEdit: boolean;
  edit: () => void;
}) {
  const { t } = useT("weekly-reports");
  const locale = useLocale();
  const scroll = useRef<HTMLDivElement>(null);
  const overview =
    report.snapshot?.overview ?? report.draft ?? emptyWeeklyOverview();
  const collection = report.snapshot?.collection ?? report.collection;
  const jump = (id: string) => {
    const target = document.getElementById(id);
    const container = scroll.current;
    if (target && container)
      container.scrollTo({
        top:
          container.scrollTop +
          target.getBoundingClientRect().top -
          container.getBoundingClientRect().top -
          60,
        behavior: matchMedia("(prefers-reduced-motion: reduce)").matches
          ? "auto"
          : "smooth",
      });
  };
  const exportReport = (snapshot: WeeklySnapshot) =>
    downloadWeeklyFile(
      `team-weekly-${week}.html`,
      weeklyReportHtml(snapshot, {
        title: t(($) => $.page.leader),
        period: weekRange(week),
        overall: t(($) => $.page.overall),
        version: t(($) => $.page.version_progress),
        onsite: t(($) => $.page.onsite),
        summary: t(($) => $.page.summary),
        progressFields: progressFields.map((field) =>
          t(($) => $.fields[field.label]),
        ),
        taskFields: taskFields.map((field) => t(($) => $.fields[field.label])),
        member: t(($) => $.fields.member),
        unit: t(($) => $.page.unit),
        missing: collection.missing.length
          ? t(($) => $.page.missing, {
              names: collection.missing.map((member) => member.name).join(", "),
            })
          : "",
      }),
    );
  return (
    <section className="w-reader w-team">
      <div className="w-document-toolbar">
        <span className="w-meta">
          <FileText />
          {t(($) => $.page.leader)}
        </span>
        <div className="w-actions w-push-right">
          {canEdit && (
            <Button variant="ghost" size="sm" onClick={edit}>
              <PencilLine />
              {t(($) => $.actions.edit)}
            </Button>
          )}
          {report.snapshot && (
            <Button size="sm" onClick={() => exportReport(report.snapshot!)}>
              <Download />
              {t(($) => $.actions.export)}
            </Button>
          )}
        </div>
      </div>
      <div className="w-document-scroll" ref={scroll}>
        <nav className="w-jump-nav" aria-label={t(($) => $.page.overall)}>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => jump("weekly-versionRows")}
          >
            {t(($) => $.page.version_progress)}
            <span className="w-count">{overview.versionRows.length}</span>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => jump("weekly-onsiteRows")}
          >
            {t(($) => $.page.onsite)}
            <span className="w-count">{overview.onsiteRows.length}</span>
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => jump("weekly-summary")}
          >
            {t(($) => $.page.summary)}
          </Button>
        </nav>
        <article className="w-document">
          <div className="w-document-heading">
            <h2>{t(($) => $.page.leader)}</h2>
            <span className="w-status">
              {t(($) =>
                report.snapshot ? $.states.generated : $.states.draft,
              )}
            </span>
          </div>
          <p className="w-meta">{weekRange(week)}</p>
          <p className="w-hint">
            {report.generatedAt && report.snapshot
              ? t(($) => $.page.snapshot_hint, {
                  time: new Date(report.generatedAt).toLocaleString(locale),
                })
              : t(($) => (canEdit ? $.page.source_hint : $.page.readonly))}
          </p>
          {canEdit || report.snapshot ? (
            <section className="w-section">
              <h3>{t(($) => $.page.overall)}</h3>
              <ProgressTables overview={overview} />
            </section>
          ) : (
            <p className="w-empty-row">{t(($) => $.page.empty_team)}</p>
          )}
          <section id="weekly-summary" className="w-section">
            <h3>{t(($) => $.page.summary)}</h3>
            <div className="w-collection">
              <span>
                {t(($) => $.page.collected, {
                  submitted: collection.reports.length,
                  total: collection.roster.length,
                })}
              </span>
              <span>
                {collection.missing.length
                  ? t(($) => $.page.missing, {
                      names: collection.missing
                        .map((member) => member.name)
                        .join(", "),
                    })
                  : ""}
              </span>
            </div>
            <p className="w-unit">{t(($) => $.page.unit)}</p>
            <TaskTable tasks={[]} collection={collection} />
          </section>
        </article>
      </div>
    </section>
  );
}

export function WeeklyReportsPage() {
  const { t } = useT("weekly-reports");
  const locale = useLocale();
  const workspace = useCurrentWorkspace();
  const wsId = workspace?.id ?? "";
  const user = useAuthStore((state) => state.user);
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const params = navigation.searchParams;
  const requestedView = params.get("view");
  const view: View =
    requestedView === "mine" || requestedView === "leader"
      ? requestedView
      : "team";
  const currentWeek = weekOf(new Date());
  const weekParam = params.get("week");
  const week = validWeek(weekParam) ? weekParam : currentWeek;
  const editing = params.get("edit") === "1" && view !== "team";
  const detail = params.get("detail") === "1" || editing;
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("all");
  const [expanded, setExpanded] = useState(false);
  const flushRef = useRef<(() => Promise<number>) | null>(null);
  const register = useCallback<RegisterSave>((save) => {
    flushRef.current = save;
  }, []);
  const weekQuery = useQuery({
    ...weeklyWeekOptions(wsId, week),
    enabled: !!wsId && view === "team",
  });
  const mineQuery = useQuery(weeklyMineOptions(wsId, week));
  const history = useQuery({
    ...weeklyHistoryOptions(wsId, user?.id ?? ""),
    enabled: !!wsId && !!user?.id && view === "mine",
  });
  const teamQuery = useQuery({
    ...weeklyTeamOptions(wsId, week),
    enabled: !!wsId && view === "leader",
  });
  const mine = mineQuery.data;
  const roster = weekQuery.data?.roster ?? [];
  const reports = weekQuery.data?.reports ?? [];
  const selectedId =
    view === "mine"
      ? user?.id
      : (params.get("member") ??
        reports.find((report) => report.published)?.userId ??
        roster[0]?.userId);
  const selected =
    view === "mine" || selectedId === user?.id
      ? mine
      : reports.find((report) => report.userId === selectedId);
  const member = roster.find((entry) => entry.userId === selectedId);
  const own = selectedId === user?.id;
  const weeks = [
    ...new Set([
      week,
      currentWeek,
      ...(history.data ?? []).map((report) => report.weekStart),
    ]),
  ]
    .sort()
    .reverse();
  const activeQuery =
    view === "leader" ? teamQuery : view === "mine" ? mineQuery : weekQuery;

  const replace = (updates: Record<string, string | null>) => {
    const next = new URLSearchParams(params);
    for (const [key, value] of Object.entries(updates)) {
      if (value === null) next.delete(key);
      else next.set(key, value);
    }
    navigation.replace(`${paths.weeklyReports()}?${next}`);
  };
  const navigate = async (updates: Record<string, string | null>) => {
    try {
      await flushRef.current?.();
      replace(updates);
    } catch {
      /* Stay on the editor that retains the unsaved content. */
    }
  };
  const close = () => replace({ edit: null });
  const reload = () => {
    flushRef.current = null;
    void activeQuery.refetch().then(() => replace({ edit: null }));
  };
  const changeView = (next: View) => {
    void navigate({ view: next, edit: null, detail: null, member: null });
  };
  const write = () => {
    void mineQuery
      .refetch()
      .then(() =>
        navigate({ view: "mine", edit: "1", detail: "1", member: null }),
      );
  };
  const weekChange = (value: string) => {
    void navigate({ week: value, edit: null, detail: null });
  };
  const visibleMembers = roster.filter((entry) => {
    const submitted = reports.some(
      (report) => report.userId === entry.userId && report.published,
    );
    return (
      entry.name.toLocaleLowerCase().includes(search.toLocaleLowerCase()) &&
      (filter === "all" || submitted === (filter === "submitted"))
    );
  });

  return (
    <div
      className={`weekly-page ${detail ? "w-detail-open" : ""} ${view === "leader" ? "w-leader-open" : ""} ${editing || expanded ? "w-expanded" : ""}`}
    >
      <PageHeader className="w-page-header">
        <FileText className="size-4 text-muted-foreground" />
        <h1 className="flex-1 text-title-sm font-semibold">
          {t(($) => $.page.title)}
        </h1>
        {view !== "leader" && !editing && (
          <Button size="sm" onClick={write}>
            <PencilLine />
            {t(($) =>
              mine?.published
                ? $.actions.edit
                : mine?.draft?.tasks.length
                  ? $.actions.continue
                  : $.actions.write,
            )}
          </Button>
        )}
      </PageHeader>
      <div className="w-page-toolbar">
        <nav className="w-tabs" aria-label={t(($) => $.page.title)}>
          {(
            [
              { key: "team", label: "team", icon: Users },
              { key: "mine", label: "mine", icon: FileText },
              { key: "leader", label: "leader", icon: FileText },
            ] as const
          ).map((tab) => (
            <Button
              key={tab.key}
              variant="ghost"
              size="sm"
              aria-pressed={view === tab.key}
              className={view === tab.key ? "selected" : ""}
              onClick={() => changeView(tab.key)}
            >
              <tab.icon />
              {t(($) => $.page[tab.label])}
            </Button>
          ))}
        </nav>
        <div className="w-week-nav">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(($) => $.actions.prev_week)}
            onClick={() => weekChange(shiftWeek(week, -1))}
          >
            <ChevronLeft />
          </Button>
          <Input
            aria-label={t(($) => $.actions.choose_week)}
            type="date"
            value={week}
            onChange={(event) => {
              if (event.target.value)
                weekChange(weekOf(new Date(`${event.target.value}T12:00:00`)));
            }}
          />
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={t(($) => $.actions.next_week)}
            onClick={() => weekChange(shiftWeek(week, 1))}
          >
            <ChevronRight />
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="w-this-week"
            aria-label={t(($) => $.actions.this_week)}
            title={t(($) => $.actions.this_week)}
            onClick={() => weekChange(currentWeek)}
          >
            <CalendarDays />
            <span>{t(($) => $.actions.this_week)}</span>
          </Button>
        </div>
      </div>
      {view === "team" && (
        <div className="w-overview">
          <span>
            {t(($) => $.page.collected, {
              submitted: reports.filter((report) => report.published).length,
              total: roster.length,
            })}
          </span>
          <span className="w-meta">
            <Lock />
            {t(($) => $.page.private)}
          </span>
        </div>
      )}
      {activeQuery.isPending ? (
        <p className="w-empty-row" role="status">
          {t(($) => $.states.loading)}
        </p>
      ) : activeQuery.isError ? (
        <div className="w-error" role="alert">
          {t(($) => $.errors.load)}
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              void activeQuery.refetch();
            }}
          >
            {t(($) => $.actions.retry)}
          </Button>
        </div>
      ) : view === "leader" && teamQuery.data ? (
        editing && teamQuery.data.canManage ? (
          <TeamEditor
            key={`${wsId}:${week}`}
            wsId={wsId}
            week={week}
            report={teamQuery.data}
            register={register}
            close={close}
            reload={reload}
          />
        ) : (
          <TeamRead
            report={teamQuery.data}
            week={week}
            canEdit={teamQuery.data.canManage}
            edit={() => {
              void teamQuery.refetch().then(() => navigate({ edit: "1" }));
            }}
          />
        )
      ) : (
        <div className="w-workspace">
          <aside className="w-report-list">
            {view === "team" ? (
              <>
                <div className="w-list-tools">
                  <label className="w-search">
                    <Search />
                    <Input
                      aria-label={t(($) => $.page.search)}
                      placeholder={t(($) => $.page.search)}
                      value={search}
                      onChange={(event) => setSearch(event.target.value)}
                    />
                  </label>
                  <select
                    aria-label={t(($) => $.page.all)}
                    value={filter}
                    onChange={(event) => setFilter(event.target.value)}
                  >
                    <option value="all">{t(($) => $.page.all)}</option>
                    <option value="submitted">
                      {t(($) => $.states.submitted)}
                    </option>
                    <option value="pending">
                      {t(($) => $.states.pending)}
                    </option>
                  </select>
                </div>
                <h2 className="w-list-title">
                  {t(($) => $.page.members)}
                  <span>{visibleMembers.length}</span>
                </h2>
                <div className="w-list-scroll">
                  {visibleMembers.map((entry) => (
                    <Button
                      variant="ghost"
                      className={`w-member-row ${entry.userId === selectedId ? "selected" : ""}`}
                      key={entry.userId}
                      onClick={() => {
                        void navigate({ member: entry.userId, detail: "1" });
                      }}
                    >
                      <span className="w-avatar" aria-hidden="true">
                        {entry.name.slice(0, 1)}
                      </span>
                      <span className="w-member-name">{entry.name}</span>
                      <Status
                        report={reports.find(
                          (report) => report.userId === entry.userId,
                        )}
                      />
                    </Button>
                  ))}
                  {!visibleMembers.length && (
                    <p className="w-empty-row">{t(($) => $.page.no_matches)}</p>
                  )}
                </div>
              </>
            ) : (
              <>
                <h2 className="w-list-title">{t(($) => $.page.history)}</h2>
                <div className="w-list-scroll">
                  {weeks.map((value) => (
                    <Button
                      variant="ghost"
                      className={`w-history-row ${week === value ? "selected" : ""}`}
                      key={value}
                      onClick={() => {
                        void navigate({ week: value, detail: "1", edit: null });
                      }}
                    >
                      <strong>{weekRange(value)}</strong>
                      <Status
                        report={(history.data ?? []).find(
                          (report) => report.weekStart === value,
                        )}
                      />
                    </Button>
                  ))}
                </div>
              </>
            )}
          </aside>
          {editing && own && mine ? (
            <PersonalEditor
              key={`${wsId}:${week}`}
              wsId={wsId}
              week={week}
              report={mine}
              register={register}
              close={close}
              reload={reload}
            />
          ) : (
            <section className="w-reader">
              <div className="w-document-toolbar">
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-mobile-back"
                  onClick={() => replace({ detail: null })}
                >
                  <ArrowLeft />
                  {t(($) => $.actions.back)}
                </Button>
                <span className="w-meta">{t(($) => $.page.title)}</span>
                <div className="w-actions w-push-right">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="w-expand-button"
                    onClick={() => setExpanded(!expanded)}
                  >
                    {expanded ? <PanelLeftOpen /> : <PanelLeftClose />}
                    {t(($) =>
                      expanded ? $.actions.collapse : $.actions.expand,
                    )}
                  </Button>
                  {own && (
                    <Button variant="ghost" size="sm" onClick={write}>
                      <PencilLine />
                      {t(($) => $.actions.edit)}
                    </Button>
                  )}
                </div>
              </div>
              <div className="w-document-scroll">
                {selected?.published || (own && selected?.draft) ? (
                  <article className="w-document">
                    <div className="w-document-heading">
                      <h2>
                        {member?.name ?? selected.authorName ?? user?.name}
                      </h2>
                      <span className="w-status">
                        {t(($) =>
                          selected.published
                            ? $.states.submitted
                            : $.states.draft,
                        )}
                      </span>
                    </div>
                    <p className="w-meta">
                      {weekRange(week)}
                      {selected.publishedAt
                        ? ` · ${new Date(selected.publishedAt).toLocaleString(locale)}`
                        : ""}
                    </p>
                    {!selected.published && (
                      <p className="w-hint">{t(($) => $.page.private)}</p>
                    )}
                    <section className="w-section">
                      <h3>{t(($) => $.page.progress)}</h3>
                      <p className="w-unit">{t(($) => $.page.unit)}</p>
                      <TaskTable
                        tasks={(selected.published ?? selected.draft)!.tasks}
                      />
                    </section>
                    <section className="w-section">
                      <h3>{t(($) => $.fields.plan)}</h3>
                      <p className="w-prose">
                        {(selected.published ?? selected.draft)!.plan}
                      </p>
                    </section>
                  </article>
                ) : (
                  <div className="w-empty">
                    <FileText />
                    <h2>{t(($) => $.page.empty)}</h2>
                    <p>{weekRange(week)}</p>
                    {own && (
                      <Button onClick={write}>
                        <PencilLine />
                        {t(($) => $.actions.write)}
                      </Button>
                    )}
                  </div>
                )}
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}
