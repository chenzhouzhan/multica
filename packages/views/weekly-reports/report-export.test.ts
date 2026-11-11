// @vitest-environment node
import { describe, expect, it } from "vitest";
import { weeklyReportHtml } from "./report-export";

describe("weekly report HTML export", () => {
  it("escapes member and progress content while keeping line breaks and workload zero", () => {
    const html = weeklyReportHtml(
      {
        overview: {
          versionName: "Logits <3>",
          versionRows: [
            {
              id: "r",
              title: "<script>alert(1)</script>",
              type: "Feature",
              lastProgress: "First\nSecond",
              thisPlan: "Test",
              note: "",
            },
          ],
          onsiteRows: [],
        },
        collection: {
          roster: [],
          missing: [],
          reports: [
            {
              member: { userId: "u", name: "A & B", avatarUrl: "" },
              content: {
                tasks: [
                  {
                    id: "t",
                    version: "3",
                    title: "API",
                    completed: "Done",
                    spent: 0,
                    remaining: null,
                    note: "",
                  },
                ],
                plan: "",
              },
              publishedAt: "",
            },
          ],
        },
      },
      {
        title: "Team report",
        period: "2026-11-09",
        overall: "Overall",
        version: "Version",
        onsite: "Onsite",
        summary: "Summary",
        progressFields: ["Task", "Type", "Last", "Plan", "Note"],
        taskFields: [
          "Version",
          "Task",
          "Completed",
          "Spent",
          "Remaining",
          "Note",
        ],
        member: "Member",
        missing: "",
        unit: "person-days",
      },
    );
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
    expect(html).toContain("First<br>Second");
    expect(html).toContain("A &amp; B");
    expect(html).toContain("<td>0</td><td>—</td>");
    expect(html.match(/<table>/g)).toHaveLength(3);
  });
});
