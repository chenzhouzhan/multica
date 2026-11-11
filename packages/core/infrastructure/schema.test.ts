// @vitest-environment node
import { describe, expect, it, vi } from "vitest";
import { parseWithFallback } from "../api/schema";
import {
  InfrastructureMetaEntrySchema,
  InfrastructureMetaFileSchema,
  InfrastructureMetaSavedSchema,
  InfrastructureConnectionSchema,
  InfrastructureConnectionsSchema,
  InfrastructureOperationSchema,
  InfrastructureNodeSchema,
  InfrastructureServiceSchema,
  InfrastructureServiceRelationsSchema,
} from "./schema";

describe("infrastructure response boundaries", () => {
  it("defaults optional metadata and discards credentials", () => {
    const row = InfrastructureConnectionSchema.parse({
      id: "manager-a",
      name: "A",
      password: "secret",
      password_ciphertext: "sealed",
    });
    expect(row.restart_grants).toEqual([]);
    expect(row.skip_tls_verify).toBe(false);
    expect(
      InfrastructureConnectionSchema.parse({
        id: "manager-b",
        skip_tls_verify: true,
      }).skip_tls_verify,
    ).toBe(true);
    expect(
      InfrastructureConnectionSchema.parse({
        id: "manager-b",
        skip_tls_verify: "true",
      }).skip_tls_verify,
    ).toBe(false);
    expect(row).not.toHaveProperty("password");
    expect(row).not.toHaveProperty("password_ciphertext");
    expect(
      InfrastructureConnectionsSchema.parse({
        connections: [],
        can_manage: "true",
      }).can_manage,
    ).toBe(false);
  });
  it("retains unfamiliar resource states and treats unknown operation states as uncertain", () => {
    expect(
      InfrastructureNodeSchema.parse({ id: 1, status: "FUTURE_STATE" }).status,
    ).toBe("FUTURE_STATE");
    expect(
      InfrastructureOperationSchema.parse({
        id: "op",
        connection_id: "manager",
        status: "FUTURE_SUCCESS",
      }).status,
    ).toBe("unknown");
  });
  it("rejects malformed identity through parseWithFallback", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(
      parseWithFallback(
        { name: "missing id" },
        InfrastructureConnectionSchema.nullable(),
        null,
        { endpoint: "infrastructure" },
      ),
    ).toBeNull();
    vi.restoreAllMocks();
  });
  it("defaults product metadata for older responses and rejects malformed relations", () => {
    const service = InfrastructureServiceSchema.parse({ id: 8, name: "App" });
    expect(service.product).toBeNull();
    expect(service.dependencies).toEqual([]);
    expect(service.external_dependencies).toEqual([]);
    expect(
      InfrastructureServiceRelationsSchema.safeParse({
        dependencies: {},
        dependents: [],
      }).success,
    ).toBe(false);
  });
  it("rejects malformed metainfo without silently replacing file content", () => {
    expect(
      InfrastructureMetaFileSchema.safeParse({
        key: "P/v/product.yaml",
        text: true,
      }).success,
    ).toBe(false);
    expect(
      InfrastructureMetaFileSchema.safeParse({
        key: "P/v/product.yaml",
        text: true,
        content: {},
      }).success,
    ).toBe(false);
    expect(
      InfrastructureMetaEntrySchema.array().safeParse([
        { name: "product.yaml", type: "INFO_YAML" },
      ]).success,
    ).toBe(false);
    expect(
      InfrastructureMetaSavedSchema.safeParse({ saved: false }).success,
    ).toBe(false);
    expect(
      InfrastructureMetaFileSchema.parse({
        key: "P/v/product.yaml",
        text: true,
        content: "",
      }).content,
    ).toBe("");
  });
});
