# Igloo Design System

> **Purpose.** This document (a) audits the **current** state of the Igloo web UI, (b)
> defines the **canonical** style guide so the look-and-feel can be recreated in the planned
> **React Native** client, and (c) lists **prioritized improvements** that steer the palette
> toward the intended *igloo / Alaska cool-color* identity.
>
> **Companion file:** [`igloo-theme.ts`](./igloo-theme.ts) — the typed token set the RN app
> consumes (`StyleSheet` + theme object). Values in this doc and that file are kept in sync.
>
> **Status:** Documentation only. Nothing in `web/` or `server/` is changed by this doc;
> the "proposed" palette and the improvements are recommendations, not yet applied.

---

## 1. TL;DR

- The web app **looks** like a polished dark media center: navy/slate surfaces, white text,
  a warm **amber** accent, generous rounded cards, subtle motion, strong accessibility.
- Under the hood there are **two parallel styling systems**, and the "official" one (shadcn
  OKLCH design tokens) is **never actually switched on** — the dark look comes almost
  entirely from hardcoded `slate-*` / `amber-*` utility classes. See §3.
- The dominant accent is **warm amber** (`#f59e0b`, ~449 usages) with scattered cyan (~37).
  That is *visually warm*, which is in tension with the requested cool "igloo" identity.
- **Recommendation (chosen):** adopt a cool **glacier** primary (ice blue/cyan/teal) with a
  **sparing amber "aurora"** highlight. Full proposed palette in §5.3; codified in
  `igloo-theme.ts`.

---

## 2. Brand & identity

| Aspect | Current state | Files |
| --- | --- | --- |
| Name | "Igloo" — personal/self-hosted media center | `web/index.html`, sidebar |
| Logo mark | An `I` glyph in a rounded square `bg-amber-500 text-slate-900` (sidebar header); favicon is an SVG igloo dome on slate | `web/src/components/app-sidebar.tsx:128`, `web/public/favicon.svg` |
| Theme-color meta | `#0f172a` (slate-900) | `web/index.html:11` |
| Splash | Centered "Igloo" / "Starting Igloo…" over a navy gradient | `web/src/assets/boot.css`, `web/index.html:17` |
| Login hero | `login-bg.webp` backdrop behind a card | `web/src/assets/images/login-bg.webp` |

**Identity gap:** the only "cold" cue today is the navy background. The brand mark and accent
are warm amber. The igloo metaphor (ice, glacier, aurora, frost) is not yet expressed in the
palette — §5 addresses this.

---

## 3. Current-state audit (read this before changing anything)

### 3.1 Two styling systems, only one truly active

1. **Design tokens (shadcn "slate", OKLCH).** `web/src/assets/styles.css` defines a full
   token set via `@theme inline` + CSS variables, with a `:root` (light) block and a `.dark`
   override block (`--background`, `--card`, `--primary`, `--accent`, `--ring`, sidebar
   variants, chart colors). This is stock shadcn output — **no igloo-specific colors**.

2. **Hardcoded utilities.** Most screens/components ignore those tokens and hardcode Tailwind
   palette classes directly: `bg-slate-900`, `bg-slate-800`, `text-white`, `text-slate-300/400`,
   `border-slate-800/50`, and the amber accent (`bg-amber-500`, `ring-amber-400`,
   `shadow-amber-500/20`).

**The critical bug-in-waiting:** the `.dark` class is **never added to `<html>`**.
`web/index.html` ships `data-app-ready="false"` with no `class="dark"`, and
`web/src/AppBoot.tsx:14` only sets `data-app-ready="true"` — nothing toggles `.dark`. So the
OKLCH tokens resolve to their **light** values. The app still *looks* dark because:

- `web/src/assets/boot.css` hardcodes `html { background:#020617 }` + a navy body gradient and
  `color:#f8fafc`; and
- components paint themselves dark with the hardcoded `slate-*` utilities.

Any generic shadcn component that *does* use tokens (e.g. a plain `Card` → `bg-card`, a default
`Button` → `bg-primary`) renders **light-mode** values on the dark canvas — e.g. a white card or
a dark-slate primary button with weak contrast. Today this is mostly masked because the media
UI overrides those classes, but it is a latent inconsistency and a trap for new components.

### 3.2 Inconsistencies & debt

- **Accent split:** amber dominates (~449) but cyan appears in ~12 files (`ProgressBar`,
  `MoviePlayerControls`, `VolumeControl`, `ChapterMenu`, `PlaylistCard`, music routes,
  some settings) — no single rule for which is used where.
- **Focus-ring color varies:** mostly `ring-amber-400`, occasionally `ring-cyan-400`, plus the
  token `ring-ring/50` on shadcn primitives → three focus looks.
- **Duplicated card chrome:** `MovieCard`, `AlbumCard`, `PlaylistCard`, etc. each re-declare the
  same hover/translate/glow/overlay strings (partly factored into the `CARD_*` constants, partly
  inline).
- **Hardcoded slate scale vs. semantic tokens:** surface/border/text values are literals, so a
  re-skin means a find-and-replace rather than a token change.

### 3.3 What's genuinely good (keep it)

- **Accessibility** is implemented intentionally and consistently (see §10).
- **Motion** is centralized and respects reduced-motion (`web/src/lib/constants.ts`, §8).
- **Component structure** is clean shadcn/radix + `cva` variants + a `cn()` merge helper.
- Visual rhythm (2:3 posters, square covers, consistent grids and section padding) is coherent.

---

## 4. Where styling lives (map)

| Concern | Source of truth |
| --- | --- |
| OKLCH design tokens (light + `.dark`) | `web/src/assets/styles.css` |
| Boot background / gradient / font / splash | `web/src/assets/boot.css`, `web/index.html` |
| Theme activation (the missing `.dark`) | `web/src/AppBoot.tsx` |
| Motion + card interaction tokens | `web/src/lib/constants.ts` |
| UI primitives (cva variants) | `web/src/components/ui/*` |
| Media cards & detail blocks | `web/src/components/MovieCard.tsx`, `AlbumCard.tsx`, `PlaylistCard.tsx`, `MovieDetailsPosterBlock.tsx` |
| App shell / nav | `web/src/components/AppShell.tsx`, `app-sidebar.tsx`, `Header.tsx` |
| Toast theme | `web/src/AppBoot.tsx` (Sonner `toastOptions`) |
| Class-merge helper | `web/src/lib/utils.ts` (`cn` = clsx + tailwind-merge) |

---

## 5. Color

### 5.1 Current OKLCH tokens (from `web/src/assets/styles.css`)

These are the **exact** values in the repo. Hex is **approximate** (for RN reference only).
Base radius token `--radius: 0.625rem`. Remember §3.1: the `.dark` column is **defined but not
active**.

| Token | `:root` (light, *active*) | `.dark` (defined, *inactive*) |
| --- | --- | --- |
| `--background` | `oklch(1 0 0)` ≈ `#FFFFFF` | `oklch(0.129 0.042 264.695)` ≈ `#0F1726` |
| `--foreground` | `oklch(0.129 0.042 264.695)` ≈ `#0F1726` | `oklch(0.984 0.003 247.858)` ≈ `#F8FAFC` |
| `--card` | `oklch(1 0 0)` ≈ `#FFFFFF` | `oklch(0.208 0.042 265.755)` ≈ `#1D2839` |
| `--primary` | `oklch(0.208 0.042 265.755)` ≈ `#1D2839` | `oklch(0.929 0.013 255.508)` ≈ `#E2E8F0` |
| `--muted-foreground` | `oklch(0.554 0.046 257.417)` ≈ `#6B7A90` | `oklch(0.704 0.04 256.788)` ≈ `#94A3B8` |
| `--destructive` | `oklch(0.577 0.245 27.325)` ≈ `#DC2626` | `oklch(0.704 0.191 22.216)` ≈ `#EF4444` |
| `--border` | `oklch(0.929 0.013 255.508)` ≈ `#E2E8F0` | `oklch(1 0 0 / 10%)` (white @10%) |
| `--ring` | `oklch(0.704 0.04 256.788)` ≈ `#94A3B8` | `oklch(0.551 0.027 264.364)` ≈ `#6B7280` |

> The full token list (popover, secondary, accent, input, sidebar*, chart 1–5) is in
> `styles.css:44-111`; the table above is the load-bearing subset.

### 5.2 De-facto palette actually rendered (hardcoded utilities)

This is what the user actually sees today:

| Role | Value(s) | Notes |
| --- | --- | --- |
| Page / canvas | `#020617` (boot) → `bg-slate-900` `#0F172A` | navy → slate |
| Card / panel | `bg-slate-900` / `bg-slate-800` `#1E293B` | |
| Border | `border-slate-800/50` | hairline |
| Text primary | `text-white` / `#F8FAFC` | |
| Text secondary / tertiary | `text-slate-300` `#CBD5E1` / `text-slate-400` `#94A3B8` | |
| **Accent (warm)** | `amber-500` `#F59E0B`, hover `amber-400` `#FBBF24` | CTAs, logo, focus rings, card glow |
| Accent (cool, scattered) | `cyan-400/500` `#22D3EE`/`#06B6D4` | player/progress/music only |
| Success / Error | `emerald-*` / `red-*` (toasts), `destructive` token | |

### 5.3 Igloo palette (current — see `igloo-theme.ts`)

Direction chosen: **cool glacier primary + sparing amber aurora.** Dark-first. **Now
implemented** in `web/src/assets/styles.css` as OKLCH tokens — the `.dark` block holds this
dark igloo palette and `:root` holds a matching light igloo palette (the app boots dark; a
theme toggle will switch them). The hex values below are the source of intent; contrast ratios
are computed against the noted background (WCAG 2.1 — AA = 4.5:1 normal text / 3:1 large+UI;
AAA = 7:1) and are guarded in CI by `web/src/test/contrast.test.ts`.

| Role | Token | Hex | Contrast check |
| --- | --- | --- | --- |
| Page canvas | `canvas` | `#0A1322` | — |
| Surface (panel) | `surface` | `#0F1A2E` | — |
| Raised (card) | `surfaceRaised` | `#15233A` | — |
| Overlay (menu/dialog) | `surfaceOverlay` | `#1B2B45` | — |
| Border subtle | `borderSubtle` | `rgba(255,255,255,0.08)` | — |
| Border strong | `borderStrong` | `#2A3C57` | — |
| Text primary | `textPrimary` | `#F8FAFC` | **~18:1** on canvas (AAA) |
| Text secondary | `textSecondary` | `#AFC0D6` | **~9.7:1** on surface (AAA) |
| Text muted | `textMuted` | `#8094AE` | **~5.1:1** on surfaceRaised (AA) † |
| **Primary (glacier)** | `primary` | `#38BDF8` | **~8.4:1** as fg on surface (AAA) |
| Primary hover / active | `primaryHover` / `primaryActive` | `#7DD3FC` / `#0EA5E9` | — |
| On-primary text | `onPrimary` | `#08131F` | **~8.7:1** on primary (AAA) |
| Cool accent (teal) | `accentTeal` | `#2DD4BF` | high on dark |
| **Aurora amber (sparing)** | `aurora` | `#F59E0B` | **~8.4:1** on surface; on-text `#08131F` ~8.7:1 |
| Success | `success` | `#34D399` | — |
| Danger | `danger` | `#F87171` | **~6.5:1** on surface (AA) |
| Info | `info` | `#38BDF8` | = primary |
| Focus ring | `focusRing` | `#38BDF8` | one ring color everywhere |

> † `textMuted` was tuned **up** from an initial seed `#708099` (which measured ~4.47:1, just
> under AA) to `#8094AE` so tertiary text clears AA on the lightest surface.

**Amber usage policy.** Amber stays, but only as a *warm aurora accent* with a clear job:
ratings / score badges, "In theaters", and occasional celebratory highlights. Everything that
is currently amber-by-default moves to **glacier**: primary CTAs, focus rings, card hover glow,
the sidebar logo, active-nav icon. This keeps a single cool identity with a deliberate warm spark.

> **Baseline screenshots:** the `docs/design/` captures are pre-migration ("before") shots.
> Recapturing them as the new igloo baseline is a deferred follow-up (best done against a
> running app once the theme toggle lands).

---

## 6. Typography

| Aspect | Value | Source |
| --- | --- | --- |
| Family | `Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif` | `web/src/assets/boot.css:21` |
| Smoothing | `-webkit-font-smoothing: antialiased; -moz-osx-font-smoothing: grayscale` | boot.css |
| Weights used | medium `500`, semibold `600`, bold `700` | components |
| Page title | `text-3xl md:text-4xl font-semibold tracking-tight text-white` | routes |
| Section heading | `text-xl md:text-2xl font-semibold tracking-tight` | routes |
| Card/poster title | `text-sm/tight font-semibold` + `line-clamp-2` | `MovieCard.tsx:81` |
| Body / meta | `text-sm text-slate-300` / `text-xs text-slate-400` | components |

**Scale** (dp ≈ rem×16): `xs 12 · sm 14 · base 16 · lg 18 · xl 20 · 2xl 24 · 3xl 30 · 4xl 36`.
`tracking-tight` (−0.02em) on headings. → `igloo-theme.ts › typography`.

**RN notes:** ship "Inter" via `expo-font`; `line-clamp-N` → `numberOfLines={N}`; RN
`letterSpacing` is **absolute dp**, not em (theme provides `tight: -0.4`).

---

## 7. Spacing & layout

- **Base unit:** Tailwind 4px scale → dp 1:1 (`igloo-theme.ts › spacing`, plus `space(step)`).
- **Screen padding:** `px-4 py-6 sm:px-6 lg:px-8` (`AppShell.tsx:46`).
- **Grid gaps:** `gap-3` (tight), `gap-4` (standard), `gap-6` (sections).
- **Responsive media grids** (observed):
  - Movies / Albums: `grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`
  - Musicians: `… lg:grid-cols-5`
  - Home "recently added": `grid-cols-[repeat(auto-fit,minmax(min(7.5rem,100%),1fr))]`
- **App shell:** sidebar `16rem` desktop / `18rem` mobile / `3rem` collapsed-icon; sticky header
  `h-14`; content area scrolls independently.

**RN notes:** there is no sidebar idiom on phones — map the sidebar nav (`Home, Movies, TV
Shows, Music, Photos, Settings` from `app-sidebar.tsx:42`) to **bottom tabs** (or a drawer);
use a TV/10-foot layout with a focus engine if a TV target is added.

---

## 8. Radii, elevation & motion

**Radii** (`igloo-theme.ts › radii`): base 10px. `sm 6 · md 8 (buttons/inputs) · lg 10 ·
xl 14 (cards/media/dialogs) · full (pills, avatars, circular play buttons)`.

**Elevation.** Web uses `shadow-sm` (cards) → `shadow-xl` (hover) → `shadow-2xl` (dialogs),
plus the colored **amber glow** on card hover (`hover:shadow-amber-500/20`). The canonical set
replaces that glow with a **glacier glow** (`elevation.glow`, `#38BDF8`). RN has no `box-shadow`
parity: use iOS `shadow*` props + Android `elevation` (presets in `igloo-theme.ts › elevation`).

**Motion** (`web/src/lib/constants.ts`, mirrored in `igloo-theme.ts › motion`):

| Token | Value |
| --- | --- |
| `MOTION_DURATION_MICRO_MS` | `150` (focus/hover) |
| `MOTION_DURATION_STANDARD_MS` | `200` (transitions, card hover/overlay) |
| `MOTION_DURATION_PAGE_MS` | `300` (page/section enter) |
| Default easing | `ease-out` ≈ `cubic-bezier(0,0,0.2,1)` |
| Reduced motion | every animated class carries a `motion-reduce:` reset |

Reusable interaction strings to reproduce in RN: `CARD_INTERACTIVE_SURFACE_CLASS` (border/shadow/
translate on hover), `CARD_MEDIA_HOVER_CLASS` (`group-hover:scale-105`), `CARD_OVERLAY_REVEAL_CLASS`,
`CARD_ACTION_REVEAL_CLASS` (play button scale/fade-in). Player chrome auto-hides after
`MOVIE_CONTROLS_IDLE_MS = 3000`.

---

## 9. Component catalog

For each, the **current** styling (quoted from source) and an **RN recreation** note. Quote
exact classes from the cited file when implementing.

### Button — `web/src/components/ui/button.tsx`
- Base: `inline-flex … gap-2 rounded-md text-sm font-medium … focus-visible:ring-[3px]
  focus-visible:ring-ring/50 disabled:opacity-50 transition-… duration-150`.
- Variants: `default` (`bg-primary`), `destructive`, `outline`, `secondary`, `ghost`, `link`,
  **`accent`** (`bg-amber-500 text-slate-900 shadow-md hover:bg-amber-400` + amber focus ring),
  **`accent-pill`** (same, `rounded-full`).
- Sizes: `default h-9 px-4` · `xs h-6` · `sm h-8` · `lg h-10 px-6` · `icon size-9` (+ `icon-xs/sm/lg`).
- **RN:** `Pressable` + theme; map `accent`/`accent-pill` to **glacier** `primary`/`onPrimary`
  (not amber) per §5.3; one focus/press ring = `focusRing`.

### Input — `web/src/components/ui/input.tsx`
- `h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-base shadow-xs …
  focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive`.
- **RN:** `TextInput`, height 36, radius `md`, border `borderStrong`, focus → `focusRing`,
  invalid → `danger`.

### Card — `web/src/components/ui/card.tsx`
- `flex flex-col gap-6 rounded-xl border bg-card py-6 text-card-foreground shadow-sm`; sub-parts
  `CardHeader/Title/Description/Action/Content/Footer` (padding `px-6`, title `font-semibold`).
- **RN:** `View` with `surfaceRaised`, radius `xl`, `elevation.card`, padding `xl`.

### Media cards
- **MovieCard** (`MovieCard.tsx`): `aspect-2/3` poster on `bg-slate-800`; container
  `rounded-xl border border-slate-800 bg-slate-900`; hover `-translate-y-1 hover:border-amber-500/50
  hover:shadow-xl hover:shadow-amber-500/20`; focus `ring-2 ring-amber-400 ring-offset-2`; bottom
  gradient `bg-linear-to-t from-black/90 via-black/50 to-transparent`; centered `size-14 rounded-full
  bg-amber-500` play button revealed on hover/focus; title `line-clamp-2 text-sm/tight`.
- **AlbumCard** (`AlbumCard.tsx`): `aspect-square`; same chrome with `amber-400` glow; play button
  `size-12` offset upward; info block `min-h-17 p-3`, musician `text-xs text-slate-400`.
- **PlaylistCard**: square cover with a fallback gradient `from-slate-700 via-slate-800 to-cyan-900/30`,
  centered text, owner pill `bg-amber-500/90`.
- **RN:** `Pressable` + `Image` (`aspectRatio` from `aspect.poster`/`aspect.square`), bottom fade via
  `expo-linear-gradient`, hover/focus → glacier border + `elevation.glow`, circular `radii.full`
  play button in `primary`/`onPrimary`. `line-clamp` → `numberOfLines`.

### App shell & navigation — `AppShell.tsx`, `app-sidebar.tsx`, `Header.tsx`
- Skip link: `sr-only focus:not-sr-only … focus:bg-amber-400` (`AppShell.tsx:26`).
- Header: `sticky top-0 z-40 h-14 border-b border-slate-800/50 bg-slate-900/95 backdrop-blur-sm`.
- Sidebar: `bg-slate-900`, logo tile `bg-amber-500`, active item `bg-slate-800 text-white`, active
  icon `text-amber-400`, inactive `text-slate-300/400`; toggle `Ctrl/Cmd+B`.
- **RN:** bottom tabs/drawer; active tint → `primary` (glacier), surfaces from tokens; header blur via
  `expo-blur`.

### Dialog / Sheet / Select / Dropdown / Tooltip / Popover — `web/src/components/ui/*`
- Radix primitives with `data-[state]` enter/exit animations (`fade/zoom/slide`, 200–300ms) and
  token surfaces; `Sheet` slides from a side (mobile nav/menus).
- **RN:** `Modal` / `react-native-screens` sheets / a menu lib; reproduce enter/exit with the
  `motion.duration` + `motion.easing` tokens; overlays use `surfaceOverlay` + `scrimStrong`.

### Feedback: Alert / Skeleton / Spinner / Toast
- `Alert` `variant: default | destructive`. `Skeleton` `animate-pulse bg-accent` (reduced-motion safe).
- `Spinner` `Loader2` `animate-spin`. Toasts via **Sonner** with hardcoded classes in
  `AppBoot.tsx` (`success` emerald, `error` red, `info` slate + amber edge).
- **RN:** shimmer/pulse Skeleton gated on reduce-motion; `ActivityIndicator`/spinner; a toast lib
  themed with `success`/`danger`/`info`. Re-tint the toast "info" edge from amber → glacier.

### Iconography
- **Lucide** (`lucide-react`); default `size-4` (16), `shrink-0`, color via `currentColor`/`text-*`.
- **RN:** `lucide-react-native` — same names; size 16 default, `color` from tokens.

---

## 10. Accessibility contract (non-negotiable — carry to RN)

Accessibility is a core product value; reproduce these, do not drop them.

| Capability | Web implementation | RN equivalent |
| --- | --- | --- |
| Live announcements | `LiveAnnouncer` (double-buffered `aria-live` polite/assertive) | `AccessibilityInfo.announceForAccessibility` |
| Skip links | `AppShell.tsx` skip-to-content; `MovieDetailsSkipLinks.tsx` per-section | Headings + `accessibilityRole`/regions; programmatic focus |
| Labels / roles / state | `aria-label`, `role`, `aria-current="page"`, `aria-pressed` throughout sidebar & player | `accessibilityLabel`, `accessibilityRole`, `accessibilityState` |
| Player keyboard map | `hooks/useVideoPlaybackKeyboard.ts` (Space/k, j/l/←/→ ±10s, ↑/↓ vol, m, f, 0/Home, Esc) | TV remote focus engine (`hasTVPreferredFocus`, next-focus) + on-screen controls |
| Hardware/remote media | `hooks/useVideoMediaSession.ts` (Media Session: play/pause/seek + metadata/artwork) | RN media-session / lockscreen controls |
| Reduced motion | `motion-reduce:` resets on every animation | `AccessibilityInfo.isReduceMotionEnabled()` gates `motion` |
| Contrast | high-contrast white-on-navy | enforced by §5.3 token ratios |

Several Playwright suites assert accessibility & console cleanliness (e.g.
`web/e2e/login.spec.ts`: accessible names, ≥12px text, tab order, no horizontal overflow) —
keep equivalent checks for RN.

---

## 11. Screen inventory (for parity)

Routes live in `web/src/routes/`; `_auth/` guards authenticated pages.

- **Public:** `/login` (card over `login-bg.webp`, email/password + show-password).
- **Home `/_auth/`:** dashboard — Watch Rooms, Latest Movies, Latest Albums, In Theaters.
- **Movies:** library grid + genre/sort/pagination/liked/playlists (`/movies`); details
  (`/movies/$id`) with cast/crew/chapters/extras + skip links; player (`/movies/$id/play`);
  playlist; in-theaters detail.
- **Music:** library tabs albums/musicians/tracks/liked/playlists (`/music`); album, musician,
  playlist; floating **AudioPlayer**.
- **Search `/_auth/search`:** unified, tabbed per entity.
- **Watch room `/_auth/watch-rooms/$id`:** synced playback (WebSocket), member list.
- **Settings:** general, libraries, playback, account, users (admin).
- **Placeholders:** `/tv-shows`, `/photos` (UI only — no scanning/playback yet).

---

## 12. React Native mapping

Adopt [`igloo-theme.ts`](./igloo-theme.ts) directly (`StyleSheet` + theme object). Web→RN parity
gaps to plan for:

| Web | RN approach |
| --- | --- |
| CSS gradients (boot bg, card bottom fade, playlist fallback) | `expo-linear-gradient` / `react-native-linear-gradient` |
| `box-shadow` + colored glow | iOS `shadow*` + Android `elevation` → `theme.elevation.*` (incl. glacier `glow`) |
| `backdrop-blur` (header, player controls, badges) | `expo-blur` / `@react-native-community/blur` |
| `line-clamp-N` | `numberOfLines={N}` |
| `aspect-2/3`, `aspect-square` | `aspectRatio` from `theme.aspect` |
| `Inter` web font | `expo-font` |
| `letterSpacing` em | absolute dp (`theme.typography.letterSpacing`) |
| Sidebar nav | bottom tabs / drawer (phone); focus-engine 10-foot layout (TV) |
| HLS `<video>` + hls.js | `react-native-video` (HLS) / a TV player; reuse stream-mode ladder from `STREAM_MODES` in `constants.ts` |
| Accessibility (§10) | RN `AccessibilityInfo` + `accessibility*` props |

---

## 13. Prioritized improvements (recommendations)

1. **Single color source of truth.** Either *(a)* actually enable dark mode — add `class="dark"`
   to `<html>` (or set it in `AppBoot.tsx` alongside `data-app-ready`) so the OKLCH `.dark` tokens
   go live — **or** *(b)* drop the unused light token block and commit to one dark theme. Then
   replace hardcoded `slate-*`/`amber-*` with semantic tokens. **(b)** is the smaller, lower-risk
   change and matches how the app is actually used today.
2. **Re-skin to the igloo palette (§5.3).** Glacier `primary` for CTAs/active/links; amber demoted
   to the *aurora* accent (ratings/in-theaters only); convert amber card glow → glacier glow.
3. **One focus-ring color.** Standardize on `focusRing` (`#38BDF8`) everywhere; remove the
   amber/cyan/`ring-ring` split.
4. **De-duplicate card chrome** into a shared `MediaCard` surface + the existing `CARD_*` tokens so
   future cards inherit hover/focus/overlay behavior.
5. **Contrast guardrails.** Keep the §5.3 ratios; add a contrast check to CI alongside the existing
   a11y Playwright assertions.
6. **(Optional) Real light theme.** Once tokens are live, make light mode a tested, switchable mode
   rather than dead CSS — useful for accessibility and daytime use.

---

## 14. Appendix — verification & maintenance

- **Token accuracy:** every value here is cross-checked against `styles.css`, `boot.css`,
  `button.tsx`, `card.tsx`, `input.tsx`, `app-sidebar.tsx`, `AppShell.tsx`, and `constants.ts`.
  OKLCH values are exact; hex of OKLCH tokens is approximate (labeled).
- **`igloo-theme.ts`** type-checks under `tsc --strict`.
- **Contrast:** ratios in §5.3 are WCAG 2.1 relative-luminance computations against the stated
  background; `textMuted` was adjusted to clear AA.
- **Keep in sync:** when web tokens or `igloo-theme.ts` change, update §5–§8 here. When adding a
  screen, update §11.
- **Visual baseline (captured):** "before" screenshots of login, home, movies grid, movie
  detail, music, settings, search, and mobile home live in [`docs/design/`](./design/) (see
  its [`README.md`](./design/README.md)). They confirm the audit (amber accent throughout) and
  are the regression oracle for the remediation plan. To refresh: run `make dev` + `cd web &&
  bun run dev`, then re-capture.
- **Remediation:** the phased plan to fix finding #1 and adopt this palette is in
  [`design-remediation-plan.md`](./design-remediation-plan.md).
