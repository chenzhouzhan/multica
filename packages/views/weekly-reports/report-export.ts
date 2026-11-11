import type { WeeklySnapshot } from "@multica/core/weekly-reports";

const escape = (value: string) =>
  value
    .replace(
      /[&<>"']/g,
      (char) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[char] ?? char,
    )
    .replaceAll("\n", "<br>");
export function downloadWeeklyFile(
  name: string,
  content: string,
  type = "text/html;charset=utf-8",
) {
  const url = URL.createObjectURL(new Blob([content], { type }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = name;
  document.body.append(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export function weeklyReportHtml(
  snapshot: WeeklySnapshot,
  labels: {
    title: string;
    period: string;
    overall: string;
    version: string;
    onsite: string;
    summary: string;
    progressFields: string[];
    taskFields: string[];
    member: string;
    missing: string;
    unit: string;
  },
) {
  const table = (headers: string[], rows: (string | number | null)[][]) =>
    `<div class="table-scroll"><table><thead><tr>${headers.map((label) => `<th>${escape(label)}</th>`).join("")}</tr></thead><tbody>${rows.map((row) => `<tr>${row.map((value) => `<td>${escape(value == null ? "—" : String(value))}</td>`).join("")}</tr>`).join("")}</tbody></table></div>`;
  const progress = (rows: typeof snapshot.overview.versionRows) =>
    table(
      labels.progressFields,
      rows.map((row) => [
        row.title,
        row.type,
        row.lastProgress,
        row.thisPlan,
        row.note,
      ]),
    );
  return `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${escape(labels.title)}</title><style>body{max-width:1440px;margin:32px auto;padding:0 24px;font:14px/1.8 system-ui,sans-serif;color:#27272b}h1{font-size:26px}h2{margin-top:32px;font-size:20px}h3{margin-top:24px;font-size:16px}.table-scroll{overflow:auto}table{border-collapse:collapse;width:100%;min-width:740px;font-size:13px}th,td{border:1px solid #ddd;padding:10px;text-align:left;vertical-align:top;overflow-wrap:anywhere}th{background:#f6f5f8}@media print{body{margin:0}table{min-width:0;font-size:10px}.table-scroll{overflow:visible}}</style></head><body><h1>${escape(labels.title)}</h1><p>${escape(labels.period)}</p><h2>${escape(labels.overall)}</h2><h3>${escape(snapshot.overview.versionName)} ${escape(labels.version)}</h3>${progress(snapshot.overview.versionRows)}<h3>${escape(labels.onsite)}</h3>${progress(snapshot.overview.onsiteRows)}<h2>${escape(labels.summary)}</h2><p>${escape(labels.unit)}</p><p>${escape(labels.missing)}</p>${table(
    [labels.member, ...labels.taskFields],
    snapshot.collection.reports.flatMap(({ member, content }) =>
      content.tasks.map((task) => [
        member.name,
        task.version,
        task.title,
        task.completed,
        task.spent,
        task.remaining,
        task.note,
      ]),
    ),
  )}</body></html>`;
}
