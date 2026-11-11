// @vitest-environment node
import { describe, expect, it } from "vitest";
import { parseWithFallback } from "../api/schema";
import {
  WeeklyReportSchema,
  WeeklyTeamSchema,
  WeeklyWeekSchema,
  encodeWeeklyOverview,
} from "./schema";
import { validWeek, weekOf, shiftWeek } from "./date";

describe("weekly report response boundary", () => {
  it("preserves zero and unknown workloads and defaults optional fields", () => {
    const result = WeeklyReportSchema.parse({
      user_id: "u1",
      week_start: "2026-11-09",
      revision: 1,
      draft: { tasks: [{ id: "r", spent: 0, remaining: null }] },
    });
    expect(result.draft?.tasks[0]).toMatchObject({
      spent: 0,
      remaining: null,
      completed: "",
    });
    expect(result.published).toBeNull();
    expect(result.draft?.plan).toBe("");
  });
  it("handles malformed roots and optional collection fields", () => {
    expect(
      parseWithFallback(null, WeeklyReportSchema.nullable(), null, {
        endpoint: "weekly",
      }),
    ).toBeNull();
    expect(
      WeeklyWeekSchema.parse({
        week_start: "2026-11-09",
        reports: "bad",
        roster: null,
      }).reports,
    ).toEqual([]);
    expect(
      parseWithFallback({}, WeeklyTeamSchema.nullable(), null, {
        endpoint: "team",
      }),
    ).toBeNull();
  });
  it("round trips free-text leader progress with stable row ids", () => {
    const overview = {
      versionName: "Logits-3.0.0",
      versionRows: [
        {
          id: "row",
          title: "Connector",
          type: "Customization",
          lastProgress: "Connector ready; app pending",
          thisPlan: "Verify",
          note: "",
        },
      ],
      onsiteRows: [],
    };
    expect(encodeWeeklyOverview(overview).version_rows[0]).toMatchObject({
      id: "row",
      last_progress: overview.versionRows[0]!.lastProgress,
    });
  });
});

it("uses Monday dates across month and year boundaries", () => {
  expect(weekOf(new Date(2026, 10, 10))).toBe("2026-11-09");
  expect(shiftWeek("2026-01-05", -1)).toBe("2025-12-29");
  expect(validWeek("2026-11-09")).toBe(true);
  expect(validWeek("2026-11-10")).toBe(false);
  expect(validWeek("2026-02-30")).toBe(false);
});
