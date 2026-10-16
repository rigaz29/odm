// How every window looks: the language and the theme ODM's settings name,
// the system's light or dark when they say "system". Each window (main, New
// Download, progress…) is its own page and asks for itself.

import { useEffect, useState } from "react";
import { api } from "./api";
import type { Appearance } from "./bindings/odm/internal/gui/models";
import { setLanguage } from "./i18n";

// useAppearance returns the appearance once ODM has said it, null before;
// the page waits for it so it never shows in the wrong language.
export function useAppearance(): Appearance | null {
  const [appearance, setAppearance] = useState<Appearance | null>(null);
  const [systemDark, setSystemDark] = useState(() => matchMedia("(prefers-color-scheme: dark)").matches);

  useEffect(() => {
    void api.appearance().then((a) => {
      setLanguage(a.language);
      setAppearance(a);
    });
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const on = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  const dark = appearance?.theme === "dark" || (appearance?.theme !== "light" && systemDark);
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
  }, [dark]);
  return appearance;
}
