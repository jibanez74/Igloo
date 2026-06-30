# Igloo Design System

This document describes the Igloo visual design system as implemented in the web
client (`web/`), explains how to recreate those styles in a React Native app using
`StyleSheet`, and records styling issues and improvement suggestions found while
documenting it.

> Source of truth for the live web tokens: `web/src/assets/styles.css` (OKLCH
> design tokens) and `web/src/assets/boot.css` (pre-React boot styles). Contrast
> is enforced by `web/src/test/contrast.test.ts`.

---

## 1. The design system

### 1.1 Identity & approach

Igloo is an "icy glacier" themed media center. The palette is a cool glacier blue
primary with a sparing warm amber ("aurora") accent, on a cool near-white canvas
(light) or deep navy canvas (dark). **Dark is the default theme.**

The system is implemented with:

- **Tailwind CSS v4** (`@import "tailwindcss"` + `@theme inline`) — no
  `tailwind.config.js`; configuration lives in CSS.
- **shadcn/ui** ("new-york" style) primitives in `web/src/components/ui`, built on
  `radix-ui` and styled with `class-variance-authority` (CVA).
- **CSS custom properties** as semantic design tokens, defined once per theme
  (`:root` = light, `.dark` = dark) and consumed through Tailwind utility classes
  like `bg-card`, `text-muted-foreground`, `ring-ring`.
- `tw-animate-css` for enter/exit animations, with a strict `motion-reduce:`
  discipline.

Theme switching is done by toggling the `dark` class on `<html>`
(`web/src/lib/theme.ts`), persisted to `localStorage` under `igloo-theme`, with an
anti-flash inline script in `index.html` that applies the stored theme before the
first paint.

### 1.2 Color tokens (semantic)

Colors are authored in **OKLCH** in `styles.css`; the hex equivalents below come
from the inline comments in that file and from `boot.css`. Tokens are paired
(`X` surface + `X-foreground` text) so foreground/background contrast is
guaranteed.

| Token | Dark (default) | Light | Role |
|-------|----------------|-------|------|
| `background` | `#0A1322` | `#F2F7FC` | App canvas |
| `foreground` | `#F8FAFC` | `#0A1322` | Primary text |
| `card` | `#15233A` | `#FFFFFF` | Raised surface |
| `card-foreground` | `#F8FAFC` | `#0A1322` | Text on cards |
| `popover` | `#1B2B45` | `#FFFFFF` | Overlay surface |
| `popover-foreground` | `#F8FAFC` | `#0A1322` | Text on overlays |
| `primary` | `#38BDF8` (glacier) | `#0369A1` | Primary actions / brand |
| `primary-foreground` | `#08131F` | `#FFFFFF` | Text on primary |
| `secondary` / `muted` / `accent` | `#0F1A2E` | `#E3EDF7` | Subtle surfaces |
| `muted-foreground` | `#8094AE` | `#475569` | Secondary text |
| `border` | `#2A3C57` | `#CBD9E8` | Borders |
| `input` | white @ 8% | `#CBD9E8` | Input borders |
| `ring` | `#38BDF8` | `#0EA5E9` | **The one focus color** |
| `destructive` | `#F87171` | `#DC2626` | Danger / delete |
| `aurora` | `#F59E0B` | `#F59E0B` | Warm accent (sparing) |
| `aurora-foreground` | `#08131F` | `#08131F` | Text on aurora |
| `success` | `#34D399` | `#059669` | Success state |
| `accent-teal` | `#2DD4BF` | `#0D9488` | Secondary accent |
| `sidebar*` | navy variants | icy variants | Sidebar chrome |
| `chart-1..5` | glacier/teal/aurora/success/danger | same families | Data viz |

Notes:
- `ring` is deliberately a single focus color across the whole app.
- Many usages apply alpha at the call site via Tailwind's `/NN` modifier
  (`bg-primary/90`, `ring-ring/50`, `bg-black/30`).

### 1.3 Typography

- **Font family**: `Inter`, then a system-UI fallback stack (defined in
  `boot.css`), with `-webkit-font-smoothing: antialiased`.
- **Scale** (Tailwind, actual usage frequency in the codebase): `text-sm` and
  `text-xs` dominate body/secondary text; `text-lg`/`text-xl`/`text-2xl` for
  section and card titles; `text-3xl`–`text-5xl` for hero/page headings.
- **Weights**: `font-medium` (controls/labels), `font-semibold` (titles).
- Headings frequently use tight tracking (`tracking-tight` / `-0.02em`) and
  `line-clamp-{n}` for truncation. Over-media text uses `drop-shadow-lg` +
  `text-white` for legibility on posters.

### 1.4 Spacing, radius, elevation

- **Radius**: base `--radius: 0.625rem` (10px). Tailwind aliases:
  `sm = radius−4px`, `md = radius−2px`, `lg = radius`, `xl = radius+4px`.
  Cards use `rounded-xl`; buttons/inputs `rounded-md`; pills `rounded-full`.
- **Spacing**: standard Tailwind 4px scale (`gap-2`, `px-4`, `py-6`, etc.).
  Cards default to `py-6` with `px-6` sections and `gap-6` between blocks.
- **Elevation**: `shadow-xs/sm/md/lg/xl/2xl`. Interactive media cards add a
  colored glow on hover (`hover:shadow-primary/20`).

### 1.5 Motion

Centralized in `web/src/lib/constants.ts` as `MOTION_*` / `CARD_*` class
constants and duration tokens:

- Durations: `MICRO = 150ms`, `STANDARD = 200ms`, `PAGE = 300ms`.
- Transitions are property-scoped (e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]`) rather
  than `transition-all`.
- **Every** animation includes a `motion-reduce:` fallback that disables or
  neutralizes it. This is a hard accessibility rule (see §1.7).
- Reusable patterns: `CARD_SURFACE_CLASS` (hover lift + glacier glow),
  `CARD_MEDIA_HOVER_CLASS` (poster zoom), `MOTION_PAGE_ENTER_CLASS`,
  `MOTION_MEDIA_OVERLAY_ENTER_CLASS`, etc.

### 1.6 Component variants (the contract)

The `Button` (`web/src/components/ui/button.tsx`) is the clearest expression of
the system. Variants: `default`, `destructive`, `outline`, `secondary`, `ghost`,
`link`, `accent`, `accent-pill`, `aurora`. Sizes: `default`, `xs`, `sm`, `lg`,
plus icon sizes `icon`, `icon-xs`, `icon-sm`, `icon-lg`. All share a base with
focus-ring, disabled opacity, and `aria-invalid` styling.

Other tokenized primitives: `Card`, `Dialog`/`Sheet`, `Select`, `DropdownMenu`,
`Tabs`, `Tooltip`, `Popover`, `Alert`, `Input`, `Checkbox`, `Skeleton`,
`Spinner`, `Sidebar`. Shared cross-component class strings (e.g. select trigger
styling, card surfaces) are exported as constants from `constants.ts`.

### 1.7 Accessibility (non-negotiable)

- A single, consistent focus ring (`ring`) with `focus-visible:ring-[3px]` /
  `ring-2 ring-offset-2`.
- `motion-reduce:` on every transition/animation.
- Contrast budget enforced in CI by `contrast.test.ts`: body text ≥ 7:1 (AAA),
  all other foreground/surface pairs ≥ 4.5:1 (AA) — in **both** themes.
- Live regions (`LiveAnnouncer`), skip links, ARIA semantics are preserved
  throughout (see `CLAUDE.md`).

---

## 2. Recreating the styles in React Native (StyleSheet)

React Native has no Tailwind, no CSS variables, no OKLCH, and no `@media`-driven
theming. The strategy is to **port the semantic tokens to a typed JS theme object,
select the active theme via React context, and consume tokens through
`StyleSheet.create`**. The structure below mirrors the web system 1:1 so the two
clients stay conceptually aligned.

### 2.1 Tokens as a typed theme object

Convert OKLCH tokens to hex/rgba (the hexes already exist as comments in
`styles.css`). RN supports hex, `rgb()`, `rgba()`, `hsl()`, and named colors —
**not** `oklch()`.

```ts
// theme/tokens.ts
export type ColorTokens = {
  background: string;
  foreground: string;
  card: string;
  cardForeground: string;
  popover: string;
  popoverForeground: string;
  primary: string;
  primaryForeground: string;
  secondary: string;
  secondaryForeground: string;
  muted: string;
  mutedForeground: string;
  accent: string;
  accentForeground: string;
  border: string;
  input: string;
  ring: string;
  destructive: string;
  aurora: string;
  auroraForeground: string;
  success: string;
  accentTeal: string;
};

export const darkColors: ColorTokens = {
  background: "#0A1322",
  foreground: "#F8FAFC",
  card: "#15233A",
  cardForeground: "#F8FAFC",
  popover: "#1B2B45",
  popoverForeground: "#F8FAFC",
  primary: "#38BDF8",
  primaryForeground: "#08131F",
  secondary: "#0F1A2E",
  secondaryForeground: "#F8FAFC",
  muted: "#0F1A2E",
  mutedForeground: "#8094AE",
  accent: "#0F1A2E",
  accentForeground: "#F8FAFC",
  border: "#2A3C57",
  input: "rgba(255,255,255,0.08)", // was oklch(1 0 0 / 8%)
  ring: "#38BDF8",
  destructive: "#F87171",
  aurora: "#F59E0B",
  auroraForeground: "#08131F",
  success: "#34D399",
  accentTeal: "#2DD4BF",
};

export const lightColors: ColorTokens = {
  background: "#F2F7FC",
  foreground: "#0A1322",
  card: "#FFFFFF",
  cardForeground: "#0A1322",
  popover: "#FFFFFF",
  popoverForeground: "#0A1322",
  primary: "#0369A1",
  primaryForeground: "#FFFFFF",
  secondary: "#E3EDF7",
  secondaryForeground: "#0A1322",
  muted: "#E3EDF7",
  mutedForeground: "#475569",
  accent: "#E3EDF7",
  accentForeground: "#0A1322",
  border: "#CBD9E8",
  input: "#CBD9E8",
  ring: "#0EA5E9",
  destructive: "#DC2626",
  aurora: "#F59E0B",
  auroraForeground: "#08131F",
  success: "#059669",
  accentTeal: "#0D9488",
};

// Non-color scales are theme-independent.
export const radius = { sm: 6, md: 8, lg: 10, xl: 14, full: 9999 };
export const spacing = { 1: 4, 2: 8, 3: 12, 4: 16, 5: 20, 6: 24, 8: 32 };
export const fontSize = {
  xs: 12, sm: 14, base: 16, lg: 18, xl: 20,
  "2xl": 24, "3xl": 30, "4xl": 36, "5xl": 48,
};
export const fontWeight = { medium: "500", semibold: "600", bold: "700" } as const;
export const duration = { micro: 150, standard: 200, page: 300 };

export type Theme = { colors: ColorTokens; radius: typeof radius;
  spacing: typeof spacing; fontSize: typeof fontSize;
  fontWeight: typeof fontWeight; duration: typeof duration };

export const darkTheme: Theme = { colors: darkColors, radius, spacing, fontSize, fontWeight, duration };
export const lightTheme: Theme = { colors: lightColors, radius, spacing, fontSize, fontWeight, duration };
```

### 2.2 Theme context (replaces the `.dark` class)

```tsx
// theme/ThemeProvider.tsx
import { createContext, useContext, useMemo, useState } from "react";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { darkTheme, lightTheme, type Theme } from "./tokens";

type Mode = "light" | "dark";
const ThemeCtx = createContext<{ theme: Theme; mode: Mode; setMode: (m: Mode) => void }>(
  { theme: darkTheme, mode: "dark", setMode: () => {} },
);

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<Mode>("dark"); // defaults dark, like web
  const setMode = (m: Mode) => {
    setModeState(m);
    void AsyncStorage.setItem("igloo-theme", m);
  };
  const theme = mode === "dark" ? darkTheme : lightTheme;
  const value = useMemo(() => ({ theme, mode, setMode }), [theme, mode]);
  return <ThemeCtx.Provider value={value}>{children}</ThemeCtx.Provider>;
}

export const useTheme = () => useContext(ThemeCtx);
```

> Hydrate the stored value on mount (mirroring `getStoredTheme`), defaulting to
> `"dark"` when absent. There is no "anti-flash" concern in RN the way there is
> in HTML, but render a splash until the stored mode is read to avoid a flicker.

### 2.3 Styling components

Because RN `StyleSheet.create` styles are static, build themed styles inside the
component with `useMemo` keyed on the theme, or use a small factory.

```tsx
// Button.tsx — port of the CVA button
import { Pressable, Text, StyleSheet } from "react-native";
import { useTheme } from "./theme/ThemeProvider";

type Variant = "default" | "destructive" | "outline" | "secondary"
  | "ghost" | "link" | "accent" | "aurora";
type Size = "default" | "sm" | "lg" | "icon";

export function Button({ variant = "default", size = "default", label, onPress, disabled }: {
  variant?: Variant; size?: Size; label: string; onPress?: () => void; disabled?: boolean;
}) {
  const { theme: t } = useTheme();
  const s = makeStyles(t);
  return (
    <Pressable
      onPress={onPress}
      disabled={disabled}
      accessibilityRole="button"
      style={({ pressed }) => [
        s.base, s[`size_${size}`], s[`variant_${variant}`],
        pressed && s.pressed,         // ~ hover:bg-*/90
        disabled && s.disabled,       // disabled:opacity-50
      ]}
    >
      <Text style={[s.label, s[`label_${variant}`]]}>{label}</Text>
    </Pressable>
  );
}

const makeStyles = (t: ReturnType<typeof useTheme>["theme"]) =>
  StyleSheet.create({
    base: { flexDirection: "row", alignItems: "center", justifyContent: "center",
      gap: t.spacing[2], borderRadius: t.radius.md },
    size_default: { height: 36, paddingHorizontal: 16 },
    size_sm: { height: 32, paddingHorizontal: 12 },
    size_lg: { height: 40, paddingHorizontal: 24 },
    size_icon: { height: 36, width: 36 },
    variant_default: { backgroundColor: t.colors.primary },
    variant_destructive: { backgroundColor: t.colors.destructive },
    variant_secondary: { backgroundColor: t.colors.secondary },
    variant_outline: { borderWidth: 1, borderColor: t.colors.border, backgroundColor: t.colors.background },
    variant_ghost: { backgroundColor: "transparent" },
    variant_link: { backgroundColor: "transparent" },
    variant_accent: { backgroundColor: t.colors.primary },
    variant_aurora: { backgroundColor: t.colors.aurora },
    pressed: { opacity: 0.9 },
    disabled: { opacity: 0.5 },
    label: { fontSize: t.fontSize.sm, fontWeight: t.fontWeight.medium },
    label_default: { color: t.colors.primaryForeground },
    label_destructive: { color: "#fff" },
    label_secondary: { color: t.colors.secondaryForeground },
    label_outline: { color: t.colors.foreground },
    label_ghost: { color: t.colors.foreground },
    label_link: { color: t.colors.primary, textDecorationLine: "underline" },
    label_accent: { color: t.colors.primaryForeground },
    label_aurora: { color: t.colors.auroraForeground },
  });
```

### 2.4 Mapping table (web → React Native)

| Web (Tailwind / CSS) | React Native equivalent |
|----------------------|-------------------------|
| `bg-card`, `text-foreground` | `backgroundColor: t.colors.card`, `color: t.colors.cardForeground` |
| `.dark` class toggle | `ThemeProvider` mode + context |
| `bg-primary/90` (alpha modifier) | precomputed rgba, or `opacity` on a wrapper |
| `rounded-xl` | `borderRadius: t.radius.xl` |
| `shadow-lg` | `elevation` (Android) + `shadowColor/Opacity/Radius/Offset` (iOS) |
| `gap-6`, `px-6`, `py-6` | `gap`, `paddingHorizontal`, `paddingVertical` (numbers) |
| `transition-* duration-200` | `Animated`/Reanimated with `duration: t.duration.standard` |
| `motion-reduce:` | check `AccessibilityInfo.isReduceMotionEnabled()` → skip animation |
| `focus-visible:ring-*` | focus is not pointer-based; use `accessibilityState`/visible focus styling for TV/remote |
| `bg-linear-to-t from-black/90` (gradient) | `expo-linear-gradient` `LinearGradient` |
| `drop-shadow-lg` on text | `textShadowColor`/`textShadowRadius`/`textShadowOffset` |
| `line-clamp-2` | `<Text numberOfLines={2}>` |
| `aspect-2/3` | `aspectRatio: 2 / 3` |
| `backdrop-blur` | `expo-blur` `BlurView` (no CSS backdrop filter) |
| `group-hover` / hover overlays | `Pressable` state on touch, or focus state on TV |

### 2.5 Things that do not translate (and what to do)

- **Hover & `group-hover`**: touch devices have no hover. Use `Pressable`'s
  pressed state, or focus state for TV/remote navigation. The poster "reveal on
  hover" overlays should become "always visible" or "reveal on focus" on TV.
- **Focus rings (`ring`)**: keyboard/pointer focus rings don't exist on touch.
  For the TV clients (a stated product target), implement a clear focused-item
  style driven by the platform's focus engine and reuse the `ring` color.
- **OKLCH & alpha modifiers**: pre-convert all tokens to hex/rgba (done above).
- **`@media (prefers-reduced-motion)`**: replace with
  `AccessibilityInfo` reduce-motion checks before running any animation —
  this preserves the web's hard `motion-reduce` rule.
- **Tailwind utility-merging (`cn`/`tailwind-merge`)**: replace with style
  arrays `[a, b, cond && c]`; later entries win, which is the RN analog.

### 2.6 Recommended libraries

- Animations: `react-native-reanimated` (port the `MOTION_*` durations/easings).
- Gradients: `expo-linear-gradient`. Blur: `expo-blur`.
- Storage: `@react-native-async-storage/async-storage` for the `igloo-theme`
  preference.
- Optional, if you prefer utility classes: **NativeWind** (Tailwind for RN) would
  let you reuse the token names and class-based mental model directly; you would
  define the same tokens in its config rather than the JS object above.

---

## 3. Styling issues found during exploration

These are concrete, fixable inconsistencies observed in the current web code.

1. **Missing referenced file `docs/igloo-theme.ts`.** `styles.css` (lines ~51,
   ~96) and `theme.ts` cite `docs/igloo-theme.ts` as the "hex source of truth,"
   but that file does not exist in `docs/`. The hexes only live in CSS comments,
   so there is no machine-readable token source. (This doc also did not exist
   until now, despite being referenced.)

2. **Theme constants duplicated in four places** that must be kept in sync by
   hand: the OKLCH tokens in `styles.css`, the hexes in `boot.css`, the inline
   anti-flash script in `index.html`, and `THEME_COLORS` in `src/lib/theme.ts`.
   The code comments acknowledge this ("keep the two in sync"), but it's a real
   drift hazard.

3. **Hardcoded, non-tokenized colors bypass the design system.** The Sonner
   toaster in `AppBoot.tsx` uses raw `bg-emerald-900/90` / `bg-red-900/90` /
   `text-emerald-100` for success/error instead of the existing `success` /
   `destructive` tokens. Card overlays (`MovieCard.tsx`, `AlbumCard.tsx`) use
   `bg-black/30`, `bg-black/40`, `text-white`, and a `from-black/90` gradient.
   Some of these (e.g. white text over a darkened poster) are legitimate, but the
   toaster colors are an inconsistency — they won't track the palette and have no
   token equivalent for an RN port.

4. **Stale comment in `styles.css`** (line ~52): "The app currently boots dark
   (AppBoot.tsx); a theme toggle will activate this." The toggle is already
   implemented (Settings → Appearance, `settings/index.lazy.tsx` calling
   `setTheme`). The comment should be updated.

5. **Toaster `success`/`error` contrast is not covered by `contrast.test.ts`.**
   The contrast test guards token pairs but not these hardcoded toast colors, so
   they could regress accessibility silently.

6. **No exported numeric scale for spacing/radius/typography.** Radius is a token
   (`--radius`) but spacing/type rely entirely on Tailwind utility literals
   scattered across components, so there's no single place that defines the
   intended scale (this matters for keeping an RN client aligned).

---

## 4. Suggestions for improvements

1. **Create a single token source of truth.** Add the `docs/igloo-theme.ts` (or
   `web/src/lib/tokens.ts`) the comments already promise: one typed object with
   hex + OKLCH per token. Generate the `styles.css` `:root`/`.dark` blocks,
   `boot.css`, the `index.html` script, and `THEME_COLORS` from it (or at least
   have them import shared hex constants), eliminating the four-way manual sync.

2. **Tokenize the toaster and any remaining raw colors.** Replace the emerald/red
   toast classes with `success`/`destructive` tokens (add `success-foreground`
   if needed) so notifications follow the theme and are portable.

3. **Extend `contrast.test.ts`** to cover the toast success/error and any other
   hardcoded UI colors, keeping the AA/AAA guarantee comprehensive.

4. **Publish the non-color scales as tokens** (spacing, font-size, font-weight,
   duration) in the same shared module, so both the web `@theme` and a future RN
   theme consume identical numbers.

5. **Fix the stale dark-boot comment** in `styles.css` now that the toggle ships.

6. **Plan the RN/TV focus model up front.** Since native TV clients are a stated
   target and the web leans on hover reveals + pointer focus rings, define the
   focus-driven equivalents (reuse the `ring` token for focused-item styling,
   convert hover reveals to focus reveals) as part of the shared system rather
   than per client.

7. **Consider NativeWind for the RN client** if you want to preserve the
   class-based authoring model and token names verbatim, reducing translation
   drift between web and native.
