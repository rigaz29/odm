// Fluent themes in Windows' accent colour, with the page's surfaces
// transparent so the window's Mica shows through, as in Windows 11's own
// apps: the navigation pane sits on Mica, the list on a lighter layer.

import { BrandVariants, Theme, createDarkTheme, createLightTheme } from "@fluentui/react-components";

function hexToRgb(hex: string): [number, number, number] | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  const v = parseInt(m[1], 16);
  return [(v >> 16) & 255, (v >> 8) & 255, v & 255];
}

function mix(a: [number, number, number], b: [number, number, number], t: number): string {
  const c = a.map((x, i) => Math.round(x + (b[i] - x) * t));
  return "#" + c.map((x) => x.toString(16).padStart(2, "0")).join("");
}

// brandRamp spreads the accent over Fluent's 16 brand shades: darker below
// shade 80 (the accent itself), lighter above.
export function brandRamp(accent: string): BrandVariants {
  const rgb = hexToRgb(accent) ?? [0, 103, 192];
  const black: [number, number, number] = [10, 10, 20];
  const white: [number, number, number] = [245, 248, 255];
  const shades = [10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 140, 150, 160] as const;
  const ramp = {} as Record<(typeof shades)[number], string>;
  for (const s of shades) {
    ramp[s] = s <= 80 ? mix(black, rgb, (s - 10) / 70) : mix(rgb, white, (s - 80) / 90);
  }
  return ramp as BrandVariants;
}

const font = '"Segoe UI Variable Text", "Segoe UI Variable", "Segoe UI", system-ui, sans-serif';

export function makeTheme(accent: string, dark: boolean): Theme {
  const brand = brandRamp(accent);
  const base = dark ? createDarkTheme(brand) : createLightTheme(brand);
  return {
    ...base,
    fontFamilyBase: font,
    // Dark Windows uses a lighter accent for text and fills.
    ...(dark ? { colorBrandForeground1: brand[110], colorBrandForegroundLink: brand[110] } : {}),
  };
}

// Windows 11 surface colours (the WinUI "layer" and "card stroke" tokens),
// set as CSS variables beside the Fluent theme.
export function surfaceVars(dark: boolean): Record<string, string> {
  return dark
    ? { "--odm-layer": "rgba(58,58,58,0.30)", "--odm-stroke": "rgba(0,0,0,0.10)", "--odm-subtle": "rgba(255,255,255,0.06)" }
    : { "--odm-layer": "rgba(255,255,255,0.50)", "--odm-stroke": "rgba(0,0,0,0.06)", "--odm-subtle": "rgba(0,0,0,0.04)" };
}
