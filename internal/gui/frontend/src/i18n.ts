// The page is written in English; its Indonesian comes from ODM's shared
// catalog (internal/i18n/id.json), which the Fyne UI reads too. Every text
// passed to t or tf must be a string literal there: internal/i18n's tests
// find them so, and check each has a translation.

import catalog from "../../../i18n/id.json";

const id: Record<string, string> = catalog;

let lang = "en";

export function setLanguage(l: string) {
  lang = l === "id" ? "id" : "en";
  if (typeof document !== "undefined") document.documentElement.lang = lang;
}

export function language(): string {
  return lang;
}

// t translates s; tf also fills %d and %s, in order.
export function t(s: string): string {
  return (lang === "id" && id[s]) || s;
}

export function tf(s: string, ...args: (string | number)[]): string {
  let i = 0;
  return t(s).replace(/%[ds]/g, () => String(args[i++] ?? ""));
}
