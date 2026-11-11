import { create } from "zustand";
import { createJSONStorage, persist } from "zustand/middleware";
import { defaultStorage } from "../platform/storage";

export type ColorTheme = "soft" | "gray";
export const APPEARANCE_STORAGE_KEY = "multica_appearance";

interface AppearanceStore {
  colorTheme: ColorTheme;
  setColorTheme: (colorTheme: ColorTheme) => void;
}

export const useAppearanceStore = create<AppearanceStore>()(
  persist(
    (set) => ({
      colorTheme: "soft",
      setColorTheme: (colorTheme) => set({ colorTheme }),
    }),
    {
      name: APPEARANCE_STORAGE_KEY,
      version: 1,
      storage: createJSONStorage(() => defaultStorage),
      partialize: ({ colorTheme }) => ({ colorTheme }),
      migrate: (saved): Pick<AppearanceStore, "colorTheme"> => ({
        colorTheme:
          saved && typeof saved === "object" && "colorTheme" in saved && saved.colorTheme === "gray"
            ? "gray"
            : "soft",
      }),
    },
  ),
);
