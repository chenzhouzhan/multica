// @vitest-environment node
import { describe, expect, it } from "vitest";
import {
  clusterHref,
  groupInfrastructureProducts,
  infrastructureHref,
  filterInfrastructureServices,
} from "./catalog";
import { InfrastructureServiceSchema } from "./schema";

const service = (
  id: number,
  product?: { type: string; name: string; version: string },
) => InfrastructureServiceSchema.parse({ id, product });

describe("cluster service catalog", () => {
  it("keeps product versions separate and groups unassigned services together", () => {
    const rows = [
      service(1, { type: "STUDIO", name: "Studio", version: "v1" }),
      service(2, { type: "STUDIO", name: "Studio", version: "v2" }),
      service(3, { type: "STUDIO", name: "Studio", version: "v1" }),
      service(4),
      service(5, {
        type: "__FALLBACK__",
        name: "Other",
        version: "__FALLBACK__",
      }),
    ];
    const groups = groupInfrastructureProducts(rows);
    expect(groups.map((group) => group.services.map((row) => row.id))).toEqual([
      [1, 3],
      [2],
      [4, 5],
    ]);
    expect(groups[2]?.product).toBeNull();
  });
  it("carries the Manager alongside cluster and service ids in navigation", () => {
    const a = new URL(
      clusterHref("/acme/infrastructure", "manager-a", 7, 8),
      "https://app.test",
    );
    const b = new URL(
      clusterHref("/acme/infrastructure", "manager-b", 7, 8),
      "https://app.test",
    );
    expect(a.searchParams.get("manager")).toBe("manager-a");
    expect(b.searchParams.get("manager")).toBe("manager-b");
    expect(a.searchParams.get("cluster")).toBe("7");
    expect(a.searchParams.get("service")).toBe("8");
  });
  it("keeps catalog filters when opening and returning from service details", () => {
    const search = new URLSearchParams({
      manager: "a",
      cluster: "7",
      q: "logits",
      health: "unhealthy",
      group: "none",
    });
    const opened = new URL(
      infrastructureHref("/acme/infrastructure", search, {
        service: 8,
        tab: "metainfo",
      }),
      "https://app.test",
    );
    const returned = new URL(
      infrastructureHref(opened.pathname, opened.searchParams, {
        service: null,
        tab: null,
      }),
      "https://app.test",
    );
    expect(returned.searchParams.toString()).toBe(search.toString());
  });
  it("filters by service metadata, health and exact product version", () => {
    const rows = [
      InfrastructureServiceSchema.parse({
        id: 1,
        name: "Logits5",
        type: "LOGITS",
        health: "HEALTHY",
        product: { type: "LOGITS", version: "v1", name: "Log analytics" },
      }),
      InfrastructureServiceSchema.parse({
        id: 2,
        name: "Logits6",
        type: "LOGITS",
        health: "WARNING",
        product: { type: "LOGITS", version: "v2", name: "Log analytics" },
      }),
      InfrastructureServiceSchema.parse({ id: 3, name: "Unknown" }),
    ];
    expect(
      filterInfrastructureServices(rows, {
        search: " log analytics ",
        health: "all",
        product: "",
      }).map((s) => s.id),
    ).toEqual([1, 2]);
    expect(
      filterInfrastructureServices(rows, {
        search: "LOGITS",
        health: "unhealthy",
        product: JSON.stringify(["LOGITS", "v2"]),
      }).map((s) => s.id),
    ).toEqual([2]);
    expect(
      filterInfrastructureServices(rows, {
        search: "",
        health: "all",
        product: "other",
      }).map((s) => s.id),
    ).toEqual([3]);
  });
});
