import { test, expect } from "@playwright/test";
import { TestApiClient } from "./fixtures";

test("weekly drafts, external edits and team snapshots work across desktop and narrow screens", async ({
  page,
}, testInfo) => {
  test.setTimeout(180000);
  const api = new TestApiClient();
  const suffix = `${Date.now()}-${testInfo.workerIndex}`;
  await api.login(`weekly-${suffix}@multica.ai`, "Weekly reporter");
  const workspace = await api.ensureWorkspace(
    "Weekly reports",
    `weekly-${suffix}`,
  );
  await api.markUserOnboarded();
  const apiBase =
    process.env.NEXT_PUBLIC_API_URL ||
    `http://localhost:${process.env.PORT || "8080"}`;
  const headers = {
    Authorization: `Bearer ${api.getToken()}`,
    "X-Workspace-ID": workspace.id,
  };
  const week = "2026-11-09";
  const mine = `${apiBase}/api/weekly-reports/${week}/mine`;
  const team = `${apiBase}/api/weekly-team-reports/${week}`;
  const content = {
    tasks: [
      {
        id: "shared-row",
        version: "Logits-3.0.0",
        title: "Agent-maintained task",
        completed: "Adapter tests passed",
        spent: 1.5,
        remaining: 0,
        note: "",
      },
    ],
    plan: "Onsite verification",
  };
  try {
    // The browser and external writers use the same persisted report contract.
    const saved = await page.request.put(mine, {
      headers,
      data: { content, expected_revision: 0 },
    });
    expect(saved.status()).toBe(200);
    await page.addInitScript((token) => {
      localStorage.setItem("multica_token", token!);
      localStorage.setItem("multica:chat:isOpen", "false");
    }, api.getToken());
    await page.setViewportSize({ width: 1600, height: 1000 });
    await page.goto(
      `/${workspace.slug}/weekly-reports?view=mine&week=${week}&edit=1&detail=1`,
      { waitUntil: "domcontentloaded" },
    );
    await expect(page.getByLabel("1: Task", { exact: true })).toHaveValue(
      content.tasks[0]!.title,
      { timeout: 90000 },
    );
    await page
      .getByLabel("1: Completed this week")
      .fill("Browser and agent share this report");
    await expect(
      page.getByRole("status").filter({ hasText: /^Saved$/ }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Submit report", exact: true })
      .click();
    await expect(
      page.getByText("Browser and agent share this report", { exact: true }),
    ).toBeVisible();
    const published = await (await page.request.get(mine, { headers })).json();
    expect(published.published.tasks[0].remaining).toBe(0);

    // An external save updates a draft without changing the submitted content.
    const external = await page.request.put(mine, {
      headers,
      data: {
        content: {
          ...content,
          tasks: [{ ...content.tasks[0], completed: "External draft update" }],
        },
        expected_revision: published.revision,
      },
    });
    expect(external.status()).toBe(200);
    const afterExternal = await (
      await page.request.get(mine, { headers })
    ).json();
    expect(afterExternal.published.tasks[0].completed).toBe(
      "Browser and agent share this report",
    );
    await page
      .getByRole("button", { name: "Team summary", exact: true })
      .click();
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await page.getByLabel("Version name", { exact: true }).fill("Logits-3.0.0");
    await page
      .getByRole("button", { name: "Add version item", exact: true })
      .click();
    await page
      .getByLabel("Version development progress 1: Work item", { exact: true })
      .fill("Skill configuration");
    await page
      .getByLabel("Version development progress 1: Last week’s progress")
      .fill("Testing");
    await page
      .getByRole("button", { name: "Add onsite requirement", exact: true })
      .click();
    await page
      .getByLabel("Onsite requirements 1: Work item", { exact: true })
      .fill("Customer connector");
    await page
      .getByLabel("Onsite requirements 1: This week’s plan")
      .fill("Onsite test");
    await expect(
      page.getByRole("status").filter({ hasText: /^Saved$/ }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Generate summary", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Export report", exact: true }),
    ).toBeVisible();
    const snapshot = await (await page.request.get(team, { headers })).json();
    expect(snapshot.snapshot.overview.version_rows[0].title).toBe(
      "Skill configuration",
    );
    expect(
      snapshot.snapshot.collection.reports[0].content.tasks[0].completed,
    ).toBe("Browser and agent share this report");
    const download = page.waitForEvent("download");
    await page
      .getByRole("button", { name: "Export report", exact: true })
      .click();
    expect((await download).suggestedFilename()).toBe(
      `team-weekly-${week}.html`,
    );
    await page.screenshot({ path: testInfo.outputPath("weekly-desktop.png") });

    for (const width of [390, 320]) {
      await page.setViewportSize({ width, height: 844 });
      await expect(
        page.getByRole("heading", { name: "Team summary", exact: true }),
      ).toBeVisible();
      expect(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      ).toBe(true);
      const clippedControls = await page
        .locator(".w-week-nav button, .w-week-nav input, .w-tabs button")
        .evaluateAll((controls) =>
          controls
            .filter((control) => {
              const rect = control.getBoundingClientRect();
              return (
                rect.left < 0 ||
                rect.right > innerWidth + 1 ||
                control.scrollWidth > control.clientWidth + 1
              );
            })
            .map(
              (control) =>
                control.getAttribute("aria-label") || control.textContent,
            ),
        );
      expect(clippedControls).toEqual([]);
      const visible = await page
        .locator(".w-progress-table tbody tr")
        .first()
        .evaluate((row) => getComputedStyle(row).display);
      expect(visible).toBe("grid");
      await page.screenshot({
        path: testInfo.outputPath(`weekly-mobile-${width}.png`),
      });
    }
    await page.getByRole("button", { name: "My reports", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "My history", exact: true }),
    ).toBeVisible();
    await page.getByRole("button", { name: "Edit", exact: true }).click();
    await expect(page.getByLabel("1: Completed this week")).toHaveValue(
      "External draft update",
    );
    await page.getByLabel("1: Notes", { exact: true }).fill("Phone review");
    await expect(
      page.getByRole("status").filter({ hasText: /^Saved$/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Update report", exact: true }),
    ).toBeVisible();
    const phoneDraft = await (await page.request.get(mine, { headers })).json();
    expect(phoneDraft.draft.tasks[0].note).toBe("Phone review");
    expect(phoneDraft.published.tasks[0].completed).toBe(
      "Browser and agent share this report",
    );
    await page.screenshot({
      path: testInfo.outputPath("weekly-mobile-editor.png"),
    });
  } finally {
    await api.cleanup();
    await api.deleteWorkspace();
  }
});
