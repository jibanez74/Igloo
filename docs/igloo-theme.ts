/**
 * Igloo design tokens — canonical theme for the React Native client.
 *
 * This is the single source of truth for the *proposed* "igloo / Alaska cool"
 * visual identity (see docs/design-system.md). It is intentionally framework-free
 * (no React Native imports) so it can be type-checked standalone and consumed by a
 * plain `StyleSheet`-based RN app:
 *
 *   import { theme } from "./igloo-theme";
 *   const styles = StyleSheet.create({
 *     card: {
 *       backgroundColor: theme.colors.surfaceRaised,
 *       borderRadius: theme.radii.xl,
 *       padding: theme.spacing.md,
 *       ...theme.elevation.card,
 *     },
 *   });
 *
 * Color direction: a cool GLACIER primary (ice blue / cyan / teal) with a sparing
 * warm AMBER "aurora" highlight. All values are dark-first (the app is dark).
 * Every text/UI color pair has been checked for WCAG contrast — see the inline
 * notes and docs/design-system.md §Color.
 *
 * NOTE: hex strings only (RN has no OKLCH). The web app currently uses OKLCH tokens
 * plus hardcoded Tailwind `slate-*`/`amber-*`; this file is the consolidated target.
 */

/* ------------------------------------------------------------------ */
/* Color                                                               */
/* ------------------------------------------------------------------ */

export const colors = {
  // Surfaces (arctic night → glacier ice, darkest to lightest)
  canvas: "#0A1322", // app/page background (was boot.css #020617 / slate canvas)
  surface: "#0F1A2E", // default panel/screen surface (~slate-900)
  surfaceRaised: "#15233A", // cards, sheets (~slate-800)
  surfaceOverlay: "#1B2B45", // menus, popovers, dialogs

  // Borders / dividers
  borderSubtle: "rgba(255,255,255,0.08)", // hairline on dark
  borderStrong: "#2A3C57", // visible divider / input border

  // Text (frost white → glacier gray)
  textPrimary: "#F8FAFC", // headings & primary copy   — ~18:1 on canvas (AAA)
  textSecondary: "#AFC0D6", // secondary copy           — ~9.7:1 on surface (AAA)
  textMuted: "#8094AE", // tertiary/meta              — ~5.1:1 on surfaceRaised (AA)
  textInverse: "#08131F", // text on light/bright fills

  // Brand — glacier cyan (primary interactive accent)
  primary: "#38BDF8", // CTAs, active states, links — ~8.4:1 as fg on surface (AAA)
  primaryHover: "#7DD3FC",
  primaryActive: "#0EA5E9",
  onPrimary: "#08131F", // text/icon on `primary`     — ~8.7:1 on primary (AAA)

  // Cool secondary accent (glacier teal) — for variety / "liked" / progress
  accentTeal: "#2DD4BF",
  onAccentTeal: "#08131F",

  // Aurora amber — WARM highlight, use SPARINGLY (ratings, "in theaters", awards).
  // Do NOT use for focus rings, primary CTAs, or card hover glow (those go glacier).
  aurora: "#F59E0B", // ~8.4:1 as fg on surface (AAA)
  onAurora: "#08131F", // ~8.7:1 on aurora (AAA)

  // Semantic states
  success: "#34D399",
  warning: "#F59E0B", // shares the aurora hue
  danger: "#F87171", // ~6.5:1 on surface (AA)
  info: "#38BDF8", // shares the glacier hue
  onSemantic: "#08131F", // dark text on any solid semantic fill

  // Focus & interaction
  focusRing: "#38BDF8", // ONE focus color everywhere (replaces mixed amber/cyan)

  // Scrims / overlays (over media)
  scrim: "rgba(0,0,0,0.30)", // hover veil on cards
  scrimStrong: "rgba(0,0,0,0.60)", // modal backdrop
} as const;

/* ------------------------------------------------------------------ */
/* Spacing — 4px base scale (matches Tailwind step * 4 = dp)           */
/* ------------------------------------------------------------------ */

export const spacing = {
  none: 0,
  xxs: 2, // tailwind 0.5
  xs: 4, // 1
  sm: 8, // 2
  md: 12, // 3   — default card padding / grid gap
  lg: 16, // 4   — standard grid gap & screen padding
  xl: 24, // 6   — section gap / lg screen padding
  "2xl": 32, // 8
  "3xl": 48, // 12
  "4xl": 64, // 16
} as const;

/** Multiply Tailwind-style steps by the 4px base when you need an ad-hoc value. */
export const space = (step: number): number => step * 4;

/* ------------------------------------------------------------------ */
/* Radii — base 10px (Tailwind --radius: 0.625rem)                     */
/* ------------------------------------------------------------------ */

export const radii = {
  sm: 6, // inputs/badges      (--radius-sm)
  md: 8, // buttons/inputs     (--radius-md)
  lg: 10, // base              (--radius / --radius-lg)
  xl: 14, // cards/dialogs      (--radius-xl)  — media cards use this
  "2xl": 18,
  full: 9999, // pills, avatars, play buttons
} as const;

/* ------------------------------------------------------------------ */
/* Typography                                                          */
/* ------------------------------------------------------------------ */

export const typography = {
  // Load "Inter" via expo-font (or link the family); platform sans is the fallback.
  fontFamily: {
    sans: "Inter",
  },
  // RN fontWeight is a string union ("400" | "500" | ... | "bold").
  fontWeight: {
    medium: "500",
    semibold: "600",
    bold: "700",
  },
  // Sizes in dp (≈ Tailwind rem * 16).
  fontSize: {
    xs: 12, // text-xs
    sm: 14, // text-sm  (most body/UI)
    base: 16, // text-base
    lg: 18, // text-lg
    xl: 20, // text-xl  (section headings)
    "2xl": 24, // text-2xl
    "3xl": 30, // text-3xl (page titles)
    "4xl": 36, // text-4xl (hero)
  },
  // Unitless multipliers; multiply by fontSize for RN `lineHeight` (in dp).
  lineHeight: {
    tight: 1.1, // poster/card titles (Tailwind text-sm/tight)
    snug: 1.25,
    normal: 1.5,
    relaxed: 1.625,
  },
  // RN letterSpacing is ABSOLUTE (dp), not em. `tracking-tight` (-0.02em) ≈ these.
  letterSpacing: {
    tight: -0.4, // headings
    normal: 0,
  },
} as const;

/* ------------------------------------------------------------------ */
/* Motion — mirrors web MOTION_DURATION_* in web/src/lib/constants.ts  */
/* ------------------------------------------------------------------ */

export const motion = {
  duration: {
    micro: 150, // focus/hover micro-feedback
    standard: 200, // most transitions, card hover/overlay
    page: 300, // page/section enter
  },
  // cubic-bezier control points (for Reanimated `Easing.bezier(...)`).
  easing: {
    standard: [0.4, 0, 0.2, 1], // ease
    out: [0, 0, 0.2, 1], // ease-out (the app's default)
    in: [0.4, 0, 1, 1], // ease-in
  },
  // Always gate non-essential animation on AccessibilityInfo.isReduceMotionEnabled()
  // (parity with the web `motion-reduce:` variants).
  respectReduceMotion: true,
} as const;

/* ------------------------------------------------------------------ */
/* Elevation — cross-platform shadow presets                          */
/* (iOS reads shadow*, Android reads `elevation`)                      */
/* ------------------------------------------------------------------ */

export const elevation = {
  card: {
    shadowColor: "#000000",
    shadowOpacity: 0.3,
    shadowRadius: 8,
    shadowOffset: { width: 0, height: 2 },
    elevation: 3,
  },
  raised: {
    shadowColor: "#000000",
    shadowOpacity: 0.4,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 6 },
    elevation: 8,
  },
  overlay: {
    shadowColor: "#000000",
    shadowOpacity: 0.5,
    shadowRadius: 24,
    shadowOffset: { width: 0, height: 12 },
    elevation: 16,
  },
  // Glacier glow — the cool replacement for the web's amber hover glow
  // (e.g. focused/active media card). Use sparingly.
  glow: {
    shadowColor: "#38BDF8",
    shadowOpacity: 0.35,
    shadowRadius: 16,
    shadowOffset: { width: 0, height: 0 },
    elevation: 0,
  },
} as const;

/* ------------------------------------------------------------------ */
/* Misc tokens                                                         */
/* ------------------------------------------------------------------ */

export const opacity = {
  disabled: 0.5,
  hoverVeil: 0.3,
  modalScrim: 0.6,
} as const;

/** Media aspect ratios (RN `aspectRatio` = width / height). */
export const aspect = {
  poster: 2 / 3, // movie posters (web `aspect-2/3`)
  square: 1, // album / playlist covers (web `aspect-square`)
  backdrop: 16 / 9,
} as const;

/* ------------------------------------------------------------------ */
/* Aggregate                                                           */
/* ------------------------------------------------------------------ */

export const theme = {
  colors,
  spacing,
  radii,
  typography,
  motion,
  elevation,
  opacity,
  aspect,
} as const;

export type Theme = typeof theme;
export type ThemeColors = typeof colors;
export type ColorToken = keyof typeof colors;
export type SpacingToken = keyof typeof spacing;
export type RadiusToken = keyof typeof radii;

export default theme;
