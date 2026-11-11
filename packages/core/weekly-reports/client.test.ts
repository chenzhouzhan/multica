// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "../api/client";
import { emptyWeeklyContent, emptyWeeklyOverview } from "./schema";

afterEach(() => vi.unstubAllGlobals());
const report = { user_id: "user", week_start: "2026-11-09", revision: 2 };
function respond(value: unknown) {
  const fetcher = vi
    .fn()
    .mockResolvedValue(
      new Response(JSON.stringify(value), {
        headers: { "Content-Type": "application/json" },
      }),
    );
  vi.stubGlobal("fetch", fetcher);
  return fetcher;
}

describe("weekly report client writes", () => {
  it("sends the workspace and revision and preserves unknown versus zero workload", async () => {
    const fetcher = respond(report);
    const client = new ApiClient("https://api.test");
    const content = {
      tasks: [
        {
          id: "row",
          version: "3.0",
          title: "Task",
          completed: "Done",
          spent: 0,
          remaining: null,
          note: "",
        },
      ],
      plan: "Next",
    };
    await client.saveWeeklyReport("workspace", "2026-11-09", content, 1);
    const options = fetcher.mock.calls[0]![1];
    expect(new Headers(options.headers).get("X-Workspace-ID")).toBe(
      "workspace",
    );
    expect(JSON.parse(options.body)).toEqual({ content, expected_revision: 1 });
  });

  it("rejects malformed or stale acknowledgements instead of marking drafts saved", async () => {
    const client = new ApiClient("https://api.test");
    for (const value of [
      null,
      {},
      { ...report, revision: "2" },
      { ...report, revision: 1 },
    ]) {
      respond(value);
      await expect(
        client.saveWeeklyReport(
          "workspace",
          "2026-11-09",
          emptyWeeklyContent(),
          1,
        ),
      ).rejects.toThrow("Invalid weekly report save response");
    }
    respond({ week_start: "2026-11-09", revision: 1, collection: {} });
    await expect(
      client.saveWeeklyTeamReport(
        "workspace",
        "2026-11-09",
        emptyWeeklyOverview(),
        1,
      ),
    ).rejects.toThrow();
  });

  it("requires the published report or generated snapshot in finalization acknowledgements", async () => {
    const client = new ApiClient("https://api.test");
    respond(report);
    await expect(
      client.submitWeeklyReport("workspace", "2026-11-09", 1),
    ).rejects.toThrow("Invalid weekly report submit response");
    respond({ week_start: "2026-11-09", revision: 2, collection: {} });
    await expect(
      client.generateWeeklyTeamReport("workspace", "2026-11-09", 1),
    ).rejects.toThrow();
  });
});
