// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

const scope = {
  ws: "workspace",
  connection: "manager",
  cluster: 7,
  service: 8,
};
const operation = {
  id: "op",
  connection_id: "manager",
  cluster_id: 7,
  service_id: 8,
  kind: "publish_images",
  status: "running",
};
function reply(value: unknown) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify(value), {
        status: 202,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});
describe("image publication API scope", () => {
  it("accepts an observed server identity for review and rejects incomplete or misbound identities", async () => {
    const value = {
      configured: false,
      host: "node",
      port: 22,
      username: "root",
      host_key: "ssh-ed25519 server-public-key",
      host_key_fingerprint: "SHA256:" + "a".repeat(43),
      context: "",
      auth_type: "password",
      requires_host_key_confirmation: true,
    };
    const input = {
      host: "node",
      port: 22,
      username: "root",
      host_key: "",
      context: "",
      auth_type: "password" as const,
      password: "test-password",
      private_key: "",
    };
    const client = new ApiClient("https://api.example.test");
    reply(value);
    expect(
      await client.saveInfrastructureExecutionAccess("ws", "manager", 7, input),
    ).toMatchObject(value);
    for (const change of [
      { host_key_fingerprint: "" },
      { host_key: "" },
      { configured: true },
      { host: "other-node" },
      { port: 2200 },
      { username: "other-user" },
    ]) {
      reply({ ...value, ...change });
      await expect(
        client.saveInfrastructureExecutionAccess("ws", "manager", 7, input),
      ).rejects.toThrow("Invalid cluster execution access save response");
    }
  });
  it("accepts a connection selection request without treating it as saved", async () => {
    const value = {
      configured: false,
      host: "node",
      port: 22,
      username: "root",
      host_key: "ssh-ed25519 public",
      context: "",
      auth_type: "password",
      requires_context_selection: true,
      context_options: [
        { name: "prod", cluster: "production" },
        { name: "qa", cluster: "test" },
      ],
    };
    const input = {
      host: "node",
      port: 22,
      username: "root",
      host_key: "ssh-ed25519 public",
      context: "",
      auth_type: "password" as const,
      password: "",
      private_key: "",
    };
    reply(value);
    const client = new ApiClient("https://api.example.test");
    expect(
      await client.saveInfrastructureExecutionAccess("ws", "manager", 7, input),
    ).toMatchObject(value);
    reply({ ...value, context_options: [] });
    await expect(
      client.saveInfrastructureExecutionAccess("ws", "manager", 7, input),
    ).rejects.toThrow("Invalid cluster execution access save response");
  });
  it("rejects operation responses for another Manager or service", async () => {
    const client = new ApiClient("https://api.example.test");
    for (const wrong of [
      { connection_id: "other-manager" },
      { service_id: 9 },
      { cluster_id: 70 },
    ]) {
      reply({ ...operation, ...wrong });
      await expect(
        client.previewInfrastructureImages(scope, {
          action: "publish_images",
          items: [],
          reason: "",
        }),
      ).rejects.toThrow("Invalid image publication preview response");
      reply({ ...operation, ...wrong });
      await expect(
        client.startInfrastructureImages(scope, "op"),
      ).rejects.toThrow("Invalid image publication submission response");
    }
  });
  it("rejects a malformed execution-access response instead of treating it as configured", async () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    reply({ configured: true, port: "22" });
    await expect(
      new ApiClient("https://api.example.test").infrastructureExecutionAccess(
        "workspace",
        "manager",
        7,
      ),
    ).rejects.toThrow("Invalid cluster execution access response");
  });
});
