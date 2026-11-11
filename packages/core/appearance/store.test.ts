// @vitest-environment node
import { beforeEach, describe, expect, it } from "vitest";
import { createJSONStorage } from "zustand/middleware";
import { APPEARANCE_STORAGE_KEY, useAppearanceStore } from "./store";

const values = new Map<string, string>();

beforeEach(() => {
  values.clear();
  useAppearanceStore.persist.setOptions({
    storage: createJSONStorage(() => ({
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => { values.set(key, value); },
      removeItem: (key) => { values.delete(key); },
    })),
  });
  useAppearanceStore.setState({ colorTheme: "soft" });
});

describe("appearance preference", () => {
  it.each([
    ["blue", "soft"],
    ["gray", "gray"],
  ])("upgrades the previous %s palette to %s", async (previous, expected) => {
    values.set(APPEARANCE_STORAGE_KEY, JSON.stringify({
      state: { colorTheme: previous }, version: 0,
    }));
    await useAppearanceStore.persist.rehydrate();
    expect(useAppearanceStore.getState().colorTheme).toBe(expected);
  });

  it("restores the selected palette after rehydration", async () => {
    useAppearanceStore.getState().setColorTheme("gray");
    const saved = values.get(APPEARANCE_STORAGE_KEY)!;
    useAppearanceStore.setState({ colorTheme: "soft" });
    values.set(APPEARANCE_STORAGE_KEY, saved);
    await useAppearanceStore.persist.rehydrate();
    expect(useAppearanceStore.getState().colorTheme).toBe("gray");
  });
});
