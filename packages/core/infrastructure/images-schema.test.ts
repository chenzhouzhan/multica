// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  InfrastructureExecutionAccessSchema,
  InfrastructureImageResourcesSchema,
} from "./images-schema";
import { InfrastructureOperationSchema } from "./schema";

const target = {
  key: "deployment/qa/worker-a/worker",
  role_type: "WORKER",
  role_name: "Worker",
  kind: "Deployment",
  namespace: "qa",
  workload: "worker-a",
  uid: "uid-a",
  generation: 7,
  container: "worker",
  image: "registry.example/worker:Li-debug_7",
  replicas: 2,
};

describe("image publication response boundaries", () => {
  it("requires trustworthy workload and container identity", () => {
    expect(
      InfrastructureImageResourcesSchema.parse({
        configured: true,
        targets: [target],
      }).targets[0],
    ).toMatchObject(target);
    for (const change of [
      { uid: "" },
      { generation: 0 },
      { container: null },
      { image: undefined },
    ]) {
      expect(
        InfrastructureImageResourcesSchema.safeParse({
          configured: true,
          targets: [{ ...target, ...change }],
        }).success,
      ).toBe(false);
    }
    expect(
      InfrastructureImageResourcesSchema.safeParse({
        configured: "true",
        targets: [],
      }).success,
    ).toBe(false);
  });
  it("discards credentials from execution access metadata", () => {
    const result = InfrastructureExecutionAccessSchema.parse({
      configured: true,
      host: "node.example",
      port: 22,
      username: "deploy",
      context: "qa",
      host_key: "ssh-ed25519 public",
      auth_type: "password",
      password: "secret",
      private_key: "private",
    });
    expect(result).not.toHaveProperty("password");
    expect(result).not.toHaveProperty("private_key");
  });
  it("keeps older restart responses readable and never turns unknown publication states into success", () => {
    const old = InfrastructureOperationSchema.parse({
      id: "op",
      connection_id: "manager",
      status: "submitted",
    });
    expect(old.kind).toBe("restart_role");
    expect(old.details).toBeNull();
    const future = InfrastructureOperationSchema.parse({
      id: "op",
      connection_id: "manager",
      status: "FUTURE_SUCCESS",
      kind: "FUTURE_ACTION",
      details: { items: [{ target: { ...target, uid: null } }] },
    });
    expect(future.status).toBe("unknown");
    expect(future.kind).toBe("unknown");
    expect(future.details).toBeNull();
  });
  it("keeps a malformed ready preview unusable rather than inventing image targets", () => {
    const result = InfrastructureOperationSchema.parse({
      id: "op",
      connection_id: "manager",
      status: "ready",
      kind: "publish_images",
      details: {
        items: [
          {
            target: { ...target, uid: "" },
            image: { source: "harbor.example/app:tag", pinned: "", digest: "" },
            status: "resolved",
          },
        ],
        steps: [],
      },
    });
    expect(result.details).toBeNull();
  });
});
