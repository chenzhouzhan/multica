import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

async function mountExample(page: Page, name: string) {
  const code = readFileSync(new URL(`../examples/plugins/${name}/ui/main.js`, import.meta.url), "utf8");
  await page.setContent('<iframe id="example" sandbox="allow-scripts"></iframe>');
  await page.evaluate((source) => {
    const frame = document.querySelector<HTMLIFrameElement>("#example")!;
    frame.srcdoc = `<!doctype html><div id="root"></div><script>
      const channel = new MessageChannel();
      globalThis.__multicaPluginBridgePortV2 = channel.port2;
      channel.port1.onmessage = ({ data }) => {
        if (data.kind !== "action") return;
        let result;
        if (data.path === "/context") result = {
          workspace: { id: "workspace", name: "Demo" },
          user: { name: "Reader" },
          issue: { id: "issue", identifier: "DEMO-1", title: "Incident" },
          config: { items: "Tests pass", signoff_template: "Checked: {{items}}", strict: true }
        };
        else if (data.path.startsWith("/issues/")) result = { title: "Incident" };
        else if (data.path === "/hooks/correlate_deploys") result = {
          status: "ok", output: { summary: "Recent deployment", deploys: [{ id: "d-4821", minutes_ago: 4, author: "Reader", commit: "abc123", blast_radius: "checkout" }] }
        };
        else if (data.path === "/hooks/request_rollback") result = {
          status: "ok", output: { status: "filed", change_id: "change-1", next_step: "Awaiting approval." }
        };
        else result = { value: null };
        channel.port1.postMessage({ id: data.id, ok: true, data: result });
      };
      ${source}
    <\/script>`;
  }, code);
  return page.frameLocator("#example");
}

test("Release Checklist initializes styles before booting and enables sign-off after checking", async ({ page }) => {
  const frame = await mountExample(page, "release-checklist");
  await expect(frame.getByText("0 / 1", { exact: true })).toBeVisible();
  await expect(frame.getByRole("button", { name: "Post sign-off" })).toBeDisabled();
  await frame.getByRole("checkbox", { name: "Tests pass" }).check();
  await expect(frame.getByText("1 / 1", { exact: true })).toBeVisible();
  await frame.getByRole("button", { name: "Post sign-off" }).click();
  await expect(frame.getByText("Sign-off posted.", { exact: true })).toBeVisible();
});

test("Deploy Sentinel starts its panel and requests rollback without sandboxed browser dialogs", async ({ page }) => {
  const manifest = JSON.parse(readFileSync(new URL("../examples/plugins/deploy-sentinel/multica.plugin.json", import.meta.url), "utf8"));
  expect(manifest.contributes.hooks.find((hook: { key: string }) => hook.key === "request_rollback").triggers).toContain("ui");
  const frame = await mountExample(page, "deploy-sentinel");
  await expect(frame.getByText("Recent deployment", { exact: true })).toBeVisible();
  await frame.getByLabel("Rollback reason (at least 20 characters)").fill("The error rate increased immediately after this deployment.");
  await frame.getByRole("button", { name: "Request rollback" }).click();
  await expect(frame.getByText("Filed change-1. Awaiting approval.", { exact: true })).toBeVisible();
});
