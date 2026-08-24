// The single machine-readable source of truth for the igloo theme
// (docs/design-system.md §2.4). Every themed color lives here as OKLCH + hex;
// scripts/generate-theme.ts renders these into the marked blocks in
// src/assets/styles.css, src/assets/boot.css, and index.html. After editing
// this module, run `bun run generate:theme`. src/test/shared/theme-drift.test.ts
// fails if the generated blocks or the OKLCH↔hex pairs fall out of sync.
//
// OKLCH values carry enough decimals to round-trip exactly to their hex
// through src/test/helpers/color.ts oklchToHex — keep that property when editing.

export type ThemeName = "light" | "dark";

export interface ThemeToken {
  /** CSS value emitted into styles.css (usually `oklch(L C H)`). */
  value: string;
  /** Uppercase #RRGGBB equivalent; omitted for values with alpha. */
  hex?: string;
  /** Emitted as the inline comment after the declaration. */
  comment?: string;
}

// Duplicated pre-boot by the generated anti-flash script in index.html.
export const THEME_STORAGE_KEY = "igloo-theme";

/** Non-color tokens that only exist on :root (shared by both themes). */
export const ROOT_ONLY_TOKENS: Record<string, ThemeToken> = {
  "--radius": { value: "0.625rem" },
};

export const THEME_TOKENS: Record<ThemeName, Record<string, ThemeToken>> = {
  light: {
    "--background": {
      value: "oklch(0.974 0.009 247.9)",
      hex: "#F2F7FC",
      comment: "icy canvas #F2F7FC",
    },
    "--foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
      comment: "navy ink #0A1322",
    },
    "--card": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "surfaceRaised #FFFFFF",
    },
    "--card-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--popover": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "surfaceOverlay #FFFFFF",
    },
    "--popover-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--primary": {
      value: "oklch(0.5 0.1193 242.7)",
      hex: "#0369A1",
      comment: "deeper glacier #0369A1 (AA on white text)",
    },
    "--primary-foreground": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "onPrimary #FFFFFF",
    },
    "--secondary": {
      value: "oklch(0.942 0.017 248)",
      hex: "#E3EDF7",
      comment: "cool surface #E3EDF7",
    },
    "--secondary-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--muted": {
      value: "oklch(0.942 0.017 248)",
      hex: "#E3EDF7",
      comment: "cool surface #E3EDF7",
    },
    "--muted-foreground": {
      value: "oklch(0.446 0.037 257.3)",
      hex: "#475569",
      comment: "cool gray #475569",
    },
    "--accent": {
      value: "oklch(0.942 0.017 248)",
      hex: "#E3EDF7",
      comment: "cool surface #E3EDF7",
    },
    "--accent-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--destructive": {
      value: "oklch(0.577 0.215 27.3)",
      hex: "#DC2626",
      comment: "danger #DC2626",
    },
    "--border": {
      value: "oklch(0.879 0.026 250)",
      hex: "#CBD9E8",
      comment: "#CBD9E8",
    },
    "--input": {
      value: "oklch(0.879 0.026 250)",
      hex: "#CBD9E8",
      comment: "#CBD9E8",
    },
    "--ring": {
      value: "oklch(0.685 0.148 237.32)",
      hex: "#0EA5E9",
      comment: "glacier #0EA5E9 — one focus color",
    },
    "--chart-1": {
      value: "oklch(0.685 0.148 237.32)",
      hex: "#0EA5E9",
      comment: "glacier",
    },
    "--chart-2": {
      value: "oklch(0.6002 0.1038 184.7)",
      hex: "#0D9488",
      comment: "teal",
    },
    "--chart-3": {
      value: "oklch(0.7686 0.1647 70.1)",
      hex: "#F59E0B",
      comment: "aurora",
    },
    "--chart-4": {
      value: "oklch(0.487 0.097 163.5)",
      hex: "#167050",
      comment: "success",
    },
    "--chart-5": {
      value: "oklch(0.577 0.215 27.3)",
      hex: "#DC2626",
      comment: "danger",
    },
    "--sidebar": {
      value: "oklch(0.954 0.015 248)",
      hex: "#E8F1FA",
      comment: "#E8F1FA",
    },
    "--sidebar-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--sidebar-primary": {
      value: "oklch(0.5 0.1193 242.7)",
      hex: "#0369A1",
      comment: "glacier #0369A1",
    },
    "--sidebar-primary-foreground": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
    },
    "--sidebar-accent": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "surfaceRaised #FFFFFF",
    },
    "--sidebar-accent-foreground": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
    },
    "--sidebar-border": {
      value: "oklch(0.879 0.026 250)",
      hex: "#CBD9E8",
    },
    "--sidebar-ring": {
      value: "oklch(0.685 0.148 237.32)",
      hex: "#0EA5E9",
      comment: "glacier",
    },
    "--aurora": {
      value: "oklch(0.7686 0.1647 70.1)",
      hex: "#F59E0B",
      comment: "amber #F59E0B — warm, sparing",
    },
    "--aurora-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "onAurora #08131F",
    },
    "--success": {
      value: "oklch(0.487 0.097 163.5)",
      hex: "#167050",
      comment: "deep glacier green #167050 — ≥4.5:1 on canvas, card, and /15 tints",
    },
    "--accent-teal": {
      value: "oklch(0.6002 0.1038 184.7)",
      hex: "#0D9488",
      comment: "#0D9488",
    },
    "--destructive-foreground": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "#FFFFFF — 4.83:1 on destructive",
    },
    "--success-foreground": {
      value: "oklch(1 0 0)",
      hex: "#FFFFFF",
      comment: "#FFFFFF — 6.06:1 on success",
    },
    "--accent-teal-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "ink #08131F — 4.99:1 on accent-teal",
    },
  },
  dark: {
    "--background": {
      value: "oklch(0.187 0.034 260.2)",
      hex: "#0A1322",
      comment: "canvas #0A1322",
    },
    "--foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
      comment: "textPrimary #F8FAFC",
    },
    "--card": {
      value: "oklch(0.256 0.048 259.9)",
      hex: "#15233A",
      comment: "surfaceRaised #15233A",
    },
    "--card-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--popover": {
      value: "oklch(0.289 0.053 259.7)",
      hex: "#1B2B45",
      comment: "surfaceOverlay #1B2B45",
    },
    "--popover-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--primary": {
      value: "oklch(0.754 0.139 232.7)",
      hex: "#38BDF8",
      comment: "glacier #38BDF8",
    },
    "--primary-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "onPrimary #08131F",
    },
    "--secondary": {
      value: "oklch(0.219 0.043 261.6)",
      hex: "#0F1A2E",
      comment: "surface #0F1A2E",
    },
    "--secondary-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--muted": {
      value: "oklch(0.219 0.043 261.6)",
      hex: "#0F1A2E",
      comment: "surface #0F1A2E",
    },
    "--muted-foreground": {
      value: "oklch(0.66 0.045 255.1)",
      hex: "#8094AE",
      comment: "textMuted #8094AE",
    },
    "--accent": {
      value: "oklch(0.219 0.043 261.6)",
      hex: "#0F1A2E",
      comment: "surface #0F1A2E",
    },
    "--accent-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--destructive": {
      value: "oklch(0.711 0.166 22.2)",
      hex: "#F87171",
      comment: "danger #F87171",
    },
    "--border": {
      value: "oklch(0.354 0.053 258.3)",
      hex: "#2A3C57",
      comment: "borderStrong #2A3C57",
    },
    "--input": {
      value: "oklch(1 0 0 / 8%)",
      comment: "borderSubtle",
    },
    "--ring": {
      value: "oklch(0.754 0.139 232.7)",
      hex: "#38BDF8",
      comment: "glacier — one focus color",
    },
    "--chart-1": {
      value: "oklch(0.754 0.139 232.7)",
      hex: "#38BDF8",
      comment: "glacier",
    },
    "--chart-2": {
      value: "oklch(0.7845 0.1325 181.9)",
      hex: "#2DD4BF",
      comment: "teal",
    },
    "--chart-3": {
      value: "oklch(0.7686 0.1647 70.1)",
      hex: "#F59E0B",
      comment: "aurora",
    },
    "--chart-4": {
      value: "oklch(0.7729 0.1535 163.2)",
      hex: "#34D399",
      comment: "success",
    },
    "--chart-5": {
      value: "oklch(0.711 0.166 22.2)",
      hex: "#F87171",
      comment: "danger",
    },
    "--sidebar": {
      value: "oklch(0.219 0.043 261.6)",
      hex: "#0F1A2E",
      comment: "surface #0F1A2E",
    },
    "--sidebar-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--sidebar-primary": {
      value: "oklch(0.754 0.139 232.7)",
      hex: "#38BDF8",
      comment: "glacier",
    },
    "--sidebar-primary-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
    },
    "--sidebar-accent": {
      value: "oklch(0.256 0.048 259.9)",
      hex: "#15233A",
      comment: "surfaceRaised",
    },
    "--sidebar-accent-foreground": {
      value: "oklch(0.984 0.003 247.9)",
      hex: "#F8FAFC",
    },
    "--sidebar-border": {
      value: "oklch(1 0 0 / 8%)",
    },
    "--sidebar-ring": {
      value: "oklch(0.754 0.139 232.7)",
      hex: "#38BDF8",
      comment: "glacier",
    },
    "--aurora": {
      value: "oklch(0.7686 0.1647 70.1)",
      hex: "#F59E0B",
      comment: "amber #F59E0B — warm, sparing",
    },
    "--aurora-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "onAurora #08131F",
    },
    "--success": {
      value: "oklch(0.7729 0.1535 163.2)",
      hex: "#34D399",
      comment: "#34D399",
    },
    "--accent-teal": {
      value: "oklch(0.7845 0.1325 181.9)",
      hex: "#2DD4BF",
      comment: "#2DD4BF",
    },
    "--destructive-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "ink #08131F — 6.76:1 on destructive",
    },
    "--success-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "ink #08131F — 9.73:1 on success",
    },
    "--accent-teal-foreground": {
      value: "oklch(0.183 0.03 251.4)",
      hex: "#08131F",
      comment: "ink #08131F — 10.04:1 on accent-teal",
    },
  },
};

/** boot.css-only colors that aren't styles.css tokens. */
export const BOOT_COLORS = {
  /** #initial-splash secondary line ("Starting Igloo..."). */
  splashMessage: { light: "#64748B", dark: "#94A3B8" },
  /** Alpha of the glacier radial tint over the body canvas gradient. */
  glacierTintAlpha: { light: 0.06, dark: 0.1 },
} as const;

/** Hex of an opaque token; throws if the token is missing or alpha-only. */
export function tokenHex(theme: ThemeName, name: string): string {
  const token = THEME_TOKENS[theme][name];
  if (!token || !token.hex) {
    throw new Error(`theme token ${name} (${theme}) has no hex value`);
  }
  return token.hex;
}
