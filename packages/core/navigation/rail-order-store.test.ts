// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { RAIL_SPACES } from "../paths/spaces";
import { resolveRailOrder, useRailOrderStore } from "./rail-order-store";

describe("resolveRailOrder", () => {
  it("uses the default order until the user reorders", () => {
    expect(resolveRailOrder(null)).toEqual(RAIL_SPACES);
  });

  it("inserts Docs into a saved layout from before Docs existed", () => {
    expect(
      resolveRailOrder(["chat", "inbox", "work", "team", "clusters", "autopilots"]),
    ).toEqual(["chat", "work", "docs", "team", "clusters", "autopilots"]);
  });

  it("adds Clusters to an order saved by the official app", () => {
    expect(resolveRailOrder(["chat", "work", "docs", "team", "autopilots"]))
      .toEqual(["chat", "work", "docs", "team", "clusters", "autopilots"]);
  });

  it("keeps a saved order", () => {
    const saved = ["chat", "work", "docs", "team", "clusters", "autopilots"];
    expect(resolveRailOrder(saved)).toEqual(saved);
  });

  it("drops unknown and repeated ids", () => {
    expect(
      resolveRailOrder([
        "chat",
        "settings",
        3,
        "chat",
        "work",
        "docs",
        "team", "clusters",
        "autopilots",
      ]),
    ).toEqual(["chat", "work", "docs", "team", "clusters", "autopilots"]);
  });

  it("drops the retired Inbox space from an order saved before it moved into Work", () => {
    expect(
      resolveRailOrder(["chat", "inbox", "work", "docs", "team", "clusters", "autopilots"]),
    ).toEqual(["chat", "work", "docs", "team", "clusters", "autopilots"]);
  });

  it("puts a space missing from the saved order after its default neighbour", () => {
    expect(resolveRailOrder(["chat", "autopilots", "work"])).toEqual([
      "chat",
      "autopilots",
      "work",
      "docs",
      "team", "clusters",
    ]);
    expect(resolveRailOrder(["chat", "team", "clusters", "autopilots"])).toEqual([
      "work",
      "docs",
      "chat",
      "team", "clusters",
      "autopilots",
    ]);
  });
});

describe("useRailOrderStore", () => {
  beforeEach(() => {
    useRailOrderStore.setState({ order: null });
  });

  it("saves a custom order", () => {
    const order = ["chat", "work", "docs", "team", "clusters", "autopilots"] as const;
    useRailOrderStore.getState().setOrder(order);
    expect(useRailOrderStore.getState().order).toEqual(order);
  });

  it("forgets the order once it is back to the default", () => {
    useRailOrderStore
      .getState()
      .setOrder(["team", "clusters", "work", "docs", "chat", "autopilots"]);
    useRailOrderStore.getState().setOrder(RAIL_SPACES);
    expect(useRailOrderStore.getState().order).toBeNull();
  });
});
