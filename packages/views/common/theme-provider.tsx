"use client";

import { useLayoutEffect, type ComponentProps } from "react";
import {
  APPEARANCE_STORAGE_KEY,
  useAppearanceStore,
} from "@multica/core/appearance";
import { ThemeProvider as BaseThemeProvider } from "@multica/ui/components/common/theme-provider";

// Apply the persisted palette before hydration, independently of light/dark mode.
const colorThemeScript = `try{var p=JSON.parse(localStorage.getItem(${JSON.stringify(APPEARANCE_STORAGE_KEY)})||'null');document.documentElement.dataset.colorTheme=p?.state?.colorTheme==='gray'?'gray':'soft'}catch{document.documentElement.dataset.colorTheme='soft'}`;

export function ThemeProvider(props: ComponentProps<typeof BaseThemeProvider>) {
  useLayoutEffect(() => {
    const apply = () => {
      document.documentElement.dataset.colorTheme =
        useAppearanceStore.getState().colorTheme === "gray" ? "gray" : "soft";
    };
    apply();
    return useAppearanceStore.subscribe(apply);
  }, []);

  return (
    <>
      <script nonce={props.nonce} dangerouslySetInnerHTML={{ __html: colorThemeScript }} />
      <BaseThemeProvider {...props} />
    </>
  );
}
