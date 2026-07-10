# Igloo Web Design System — UI Development Guideline

This document is the development guideline for the Igloo web client (`web/`). It
describes the visual system as implemented, the conventions every new UI change
must follow, and the issues found in the July 2026 audit (§4) with prioritized
improvement suggestions (§5).

Scope: the web client only. (An earlier revision of this document doubled as an
Android TV / Jetpack Compose port guide; that content was removed deliberately —
recover it from git history if a TV port resumes.)

**How to use this doc**

- Building UI? Read §1 (foundations) and §3 (the pattern for your surface).
- Touching styles, tokens, or themes? Read §2.
- Code comments cite sections as `design-system §N.N` — before renumbering any
  heading here, run `grep -rn "design-system" web/src` and update the citations.

**Enforcement map** — most rules in this doc are guarded by a test or lint rule.
If you change a guarded behavior, the suite tells you; if you add a new rule to
this doc, prefer adding a guard too.

| Rule | Guarded by |
|---|---|
| Token contrast: body text ≥ 7:1 (AAA), all other fg/surface pairs ≥ 4.5:1 (AA), both themes | `web/src/test/contrast.test.ts` |
| Theme constants stay in sync across `styles.css` / `boot.css` / `index.html` / `theme.ts` | `web/src/test/theme-drift.test.ts` |
| Every shared motion constant carries a `motion-reduce:` escape; every `ui/*.tsx` transition has `motion-reduce:transition-none` | `web/src/test/motion-contracts.test.ts` |
| No raw `slate-*` / `amber-*` / `red-*` / `emerald-*` palette classes | ESLint `no-restricted-syntax` (error) in `web/eslint.config.js` |
| Class order, duplicates, unknown/conflicting classes | `eslint-plugin-better-tailwindcss` |
| Input styling contracts | `web/src/test/input-styles.test.ts` |
| Shared class-string constants keep their contracts | `web/src/test/constants-contracts.test.ts` |

---

## 1. Foundations

### 1.1 Identity & approach

Igloo is an "icy glacier" themed media center. The palette is a cool glacier
blue primary with a sparing warm amber ("aurora") accent, on a deep navy canvas
(dark, the default) or a cool near-white canvas (light).

The system is implemented with:

- **Tailwind CSS v4** — configuration lives in CSS (`@theme inline` in
  `web/src/assets/styles.css`); there is no `tailwind.config.js`.
- **shadcn/ui** ("new-york" style) primitives in `web/src/components/ui`,
  built on the consolidated `radix-ui` package and styled with `cn()`
  (`clsx` + `tailwind-merge`, from `web/src/lib/utils.ts`).
- **Semantic CSS custom properties** in OKLCH, defined once per theme
  (`:root` = light, `.dark` = dark) and consumed through utilities like
  `bg-card`, `text-muted-foreground`, `ring-ring`.
- **`tw-animate-css`** for enter/exit animations, with a strict
  `motion-reduce:` discipline (§1.5).
- **lucide-react** for all icons.

Theme switching toggles the `dark` class on `<html>` (`web/src/lib/theme.ts`),
persisted under the `igloo-theme` localStorage key, with an anti-flash inline
script in `index.html` (§2.4). Default is dark whenever no stored value exists.

### 1.2 Color tokens

Colors are authored in **OKLCH** in `styles.css` with hex equivalents in inline
comments. Tokens are paired (`X` surface + `X-foreground` text) so contrast is
guaranteed; every pair is verified in both themes by `contrast.test.ts`.

| Token | Dark (default) | Light | Role |
|-------|----------------|-------|------|
| `background` | `#0A1322` | `#F2F7FC` | App canvas |
| `foreground` | `#F8FAFC` | `#0A1322` | Primary text |
| `card` / `card-foreground` | `#15233A` / `#F8FAFC` | `#FFFFFF` / `#0A1322` | Raised surface |
| `popover` / `popover-foreground` | `#1B2B45` / `#F8FAFC` | `#FFFFFF` / `#0A1322` | Overlay surface |
| `primary` | `#38BDF8` (glacier) | `#0369A1` | Brand / primary actions |
| `primary-foreground` | `#08131F` | `#FFFFFF` | Text on primary |
| `secondary` / `muted` / `accent` | `#0F1A2E` | `#E3EDF7` | Subtle surfaces |
| `muted-foreground` | `#8094AE` | `#475569` | Secondary text |
| `border` | `#2A3C57` | `#CBD9E8` | Borders |
| `input` | white @ 8% | `#CBD9E8` | Input borders |
| `ring` | `#38BDF8` | `#0EA5E9` | **The one focus color** |
| `destructive` / `-foreground` | `#F87171` / `#08131F` | `#DC2626` / `#FFFFFF` | Danger / delete |
| `aurora` / `-foreground` | `#F59E0B` / `#08131F` | same | Warm accent (sparing) |
| `success` / `-foreground` | `#34D399` / `#08131F` | `#059669` / `#08131F` | Success state |
| `accent-teal` / `-foreground` | `#2DD4BF` / `#08131F` | `#0D9488` / `#08131F` | Secondary accent |
| `sidebar` (+ its own fg/primary/accent/border/ring set) | `#0F1A2E` | `#E8F1FA` | Sidebar chrome |
| `chart-1..5` | glacier / teal / aurora / success / danger | same families | Data viz |

Rules:

- **Use semantic tokens, never raw palette classes.** `bg-red-500`,
  `text-slate-400`, `bg-emerald-900` are banned at ESLint error level. The
  sanctioned exceptions are `src/lib/input-styles.ts` and
  `src/routes/login.lazy.tsx` (the intentionally light "frosted glass" input
  treatment that stays light-on-dark in both themes).
- `aurora` is identical in both themes and is deliberately **sparing**: rating
  badges and rare highlights, not a general accent.
- Alpha is applied at the call site with the `/NN` modifier
  (`bg-primary/90`, `ring-ring/50`, `border-destructive/25`).
- **Over-media colors are an intentional exception.** Text and overlays that
  sit on posters/backdrops use literal black/white — `bg-black/30`–`/40` dim
  overlays, `bg-linear-to-t from-black/90` scrims, `text-white` titles,
  `shadow-black/30`. These must not track the theme (a poster doesn't change
  with the theme); keep them literal, don't hunt for tokens.
- One deliberate brand exception exists: `SPOTIFY_BRAND_TEXT_CLASS` in
  `lib/constants.ts` (Spotify green, documented there).
- Known caveat: light-theme `text-success` on `background` is only ~3.5:1 —
  fine for icons/large text, not for small body text (§4 item 8).

### 1.3 Typography

- **Font stack**: `Inter, ui-sans-serif, system-ui, -apple-system,
  BlinkMacSystemFont, "Segoe UI", sans-serif`, declared once on `body` in
  `web/src/assets/boot.css` (with antialiasing). **Inter is not actually
  loaded** — there is no `@font-face`, package, or `<link>` — so the app
  renders in the platform system font today (§4 item 2). Do not declare other
  font families in components.
- **Scale**: Tailwind's default type scale as utility literals; there are no
  custom font-size tokens. In practice: `text-sm`/`text-xs` for body and
  secondary text, `text-lg`–`text-2xl` for section and card titles,
  `text-3xl`–`text-5xl` for page/hero headings.
- **Weights**: `font-medium` for controls/labels, `font-semibold` for titles.
- Headings use `tracking-tight`; truncation uses `line-clamp-{n}` (cards
  clamp titles at 2 lines). Text over media adds `drop-shadow-lg` +
  `text-white` (§1.2 over-media exception).
- Numeric readouts that update (player time) use `tabular-nums`.

### 1.4 Spacing, radius, elevation

- **Radius**: `--radius: 0.625rem` (10px) is the only non-color CSS token.
  Aliases: `rounded-sm` = radius−4px, `md` = −2px, `lg` = radius, `xl` = +4px.
  Convention: cards `rounded-xl`, buttons/inputs `rounded-md`, chips/pills
  `rounded-full`.
- **Spacing**: the standard Tailwind 4px scale as utility literals (not
  tokenized). Rhythm in practice: `gap-4` in poster grids, `gap-6` between
  card blocks, page sections `mt-6 md:mt-8`, shell content padding
  `px-4 py-6 sm:px-6 lg:px-8`.
- **Aspect ratios are part of the design vocabulary**: `aspect-2/3` movie
  posters, `aspect-square` album covers and musician thumbs, `aspect-21/9`
  detail-page backdrops (clamped `max-h-[min(42vh,22rem)]` at `md+`),
  `aspect-video` trailers/extras.
- **Elevation**: `shadow-xs`–`shadow-2xl`. Interactive media cards add a
  glacier glow on hover (`hover:shadow-xl hover:shadow-primary/20`); the
  EmptyState orb uses `shadow-primary/5`.

### 1.5 Motion

All shared motion lives in `web/src/lib/constants.ts` as exported class-string
constants ("motion tokens"). Do not hand-write one-off transition stacks when a
constant exists — and when you create a new recurring one, add it there so
`motion-contracts.test.ts` guards it.

- **Durations**: 150ms micro (controls), 200ms standard (surfaces, fades),
  300ms page enter — exported both as numbers (`MOTION_DURATION_*_MS`) and in
  the class constants. Easing is `ease-out` (exits `ease-in`).
- **Transitions enumerate properties** — e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]` —
  never `transition-all`.
- **Enter/exit animations** come from `tw-animate-css`:
  `animate-in fade-in slide-in-from-bottom-2 fill-mode-both` for page/section
  entrances (`MOTION_PAGE_ENTER_CLASS`, `MOTION_SECTION_ENTER_CLASS`, and the
  `delay-75`/`delay-150` staggered `_DELAYED_` variants), `animate-out` +
  `fade-out-0` for exits. There are no custom `@keyframes` in `styles.css`.
- **The motion-reduce contract (hard rule)**: every animation/transition ships
  a `motion-reduce:` escape — `motion-reduce:transition-none`,
  `motion-reduce:animate-none`, plus end-state resets
  (`motion-reduce:opacity-100 motion-reduce:translate-y-0
  motion-reduce:scale-100`) so reduced-motion users see the final state, not a
  broken mid-state. Guarded by `motion-contracts.test.ts`, which also scans
  every `components/ui/*.tsx` for unescaped transitions.
- Common vocabulary beyond entrances: `group-hover:opacity-100` overlay
  reveals, `group-hover:scale-105` poster zoom, `hover:-translate-y-1` card
  lift, `backdrop-blur-sm` on sticky/glass chrome, `bg-linear-to-*` gradients
  (poster scrims, backdrop fades, the EmptyState orb), and tab-panel
  cross-fades via `useContentFadeTransition` (`CONTENT_FADE_*`, 200ms).

### 1.6 Component variants — the contract

The `Button` (`web/src/components/ui/button.tsx`) is the clearest expression
of the system. Beyond stock shadcn (`default`, `destructive`, `outline`,
`secondary`, `ghost`, `link`) it adds Igloo variants: **`accent`** (primary +
`shadow-md`), **`accent-pill`** (rounded-full primary), **`aurora`** (amber).
Extra sizes: `xs`, and icon sizes `icon-xs`/`icon-sm`/`icon`/`icon-lg`. The
base string carries the focus ring, disabled opacity, `aria-invalid` styling,
a property-scoped 150ms transition with `motion-reduce:transition-none`, and
stamps `data-variant`/`data-size`.

- **Tabs share one look** (`web/src/components/ui/tabs.tsx`): a bordered
  `bg-muted/50` list; the active trigger is a glacier primary-fill pill
  (`data-[state=active]:bg-primary … shadow-primary/20`). Library pages layer
  responsive grid sizing on top via `LIBRARY_TABS_LIST_CLASS` /
  `LIBRARY_TAB_TRIGGER_CLASS` from `constants.ts`. Tab state lives in URL
  search params (validated in `types/route-search.ts`), never local state.
- **cva policy**: only `button.tsx`, `alert.tsx`, and `sidebar.tsx` use
  `class-variance-authority` (as shipped by shadcn). Everything else composes
  plain `cn(...)`. Don't introduce cva into new components; either add a
  variant to an existing cva component or export a class-string constant
  (§2.3).
- **When to add a variant vs. a constant**: a new *look* for an existing
  primitive (e.g. another Button treatment) → add a cva variant next to its
  siblings. A *cross-component* treatment (card chrome, motion, focus) → an
  exported constant in `lib/constants.ts`.
- All primitives stamp `data-slot="..."`; tests and cross-component behavior
  rely on these (e.g. `SELECT_CONTENT_SLOT_SELECTOR` keeps dialogs open while
  interacting with portaled selects). Keep them when customizing.

### 1.7 Accessibility — non-negotiable

- **One focus color.** `--ring` is glacier in both themes. Inline (non-shadcn)
  interactive elements use `FOCUS_VISIBLE_RING_CLASS` from `constants.ts`
  (`focus-visible:ring-2 ring-ring offset-2`); shadcn primitives ship the
  newer `focus-visible:ring-[3px] ring-ring/50` style. Two ring *weights*, one
  color — verified visible over posters and backdrops in both themes (§4
  item 5 tracks unifying them). Media cards add `focus-within:ring-2` on the
  `<article>` so the whole card shows focus.
- **Hover/focus parity.** Every `group-hover` reveal pairs with
  `group-focus-within` (cards) or `focus-visible` (rows) so keyboard users get
  the same affordances. Never gate an action behind hover alone.
- **Contrast budget** (CI-enforced, both themes): body text ≥ 7:1 (AAA),
  every other foreground/surface token pair ≥ 4.5:1 (AA).
- **Native elements over ARIA re-implementations.** Sliders are transparent
  native `<input type="range">` overlaying a styled track (`ProgressBar.tsx`,
  `VolumeControl.tsx`) — custom `role="slider"` divs are inert under iOS
  VoiceOver. Same spirit elsewhere: real `<button>`, `<article>`, labels.
  Don't put `disabled` on media transport buttons (breaks VoiceOver focus);
  use `aria-disabled` + guards if needed.
- **Announcements**: `LiveAnnouncer` (double-buffered dual `role="status"`
  regions so repeated messages re-announce) for async state changes; sections
  announce their load/empty/error summaries.
- **Skip links**: a global "Skip to content" in `AppShell` targeting `#main`,
  plus per-page section skip navs on long pages
  (`MovieDetailsSkipLinks` pattern, `sr-only focus-within:not-sr-only`).
- **Labels everywhere**: icon-only buttons get `aria-label`; decorative
  icons/images get `aria-hidden="true"`/`alt=""`; cards carry a full
  `aria-label` ("Play Uncut Gems 2019"); toggles use `aria-pressed`; nav uses
  `aria-current`; sections use `role="region" aria-labelledby`.

---

## 2. Working with the stack

### 2.1 Tailwind v4 conventions

- Configuration is CSS-first: `styles.css` starts with `@import "tailwindcss"`
  + `@import "tw-animate-css"`, declares `@custom-variant dark (&:is(.dark *))`
  (class-based dark mode), and maps every token into Tailwind via
  `@theme inline` (`--color-X: var(--X)`). There is no `tailwind.config.js`.
- The only global CSS: `@layer base` applies `border-border outline-ring/50`
  to `*` and `bg-background text-foreground` to `body`. Keep `styles.css`
  minimal — component styling belongs in components.
- Gradients use the v4 syntax (`bg-linear-to-t`, `bg-linear-to-br`).
- Scrollbar styling uses the v4 core utilities (`scrollbar-thin`,
  `scrollbar-thumb-primary/50`) on horizontal media rails.
- `eslint-plugin-better-tailwindcss` enforces class order, duplicates, and
  unknown classes; write class lists in its canonical order (the lint
  autofixes).

### 2.2 shadcn/ui usage

- Style **"new-york"**, `baseColor: slate`, CSS variables on, lucide icons
  (`web/components.json`). Components import from the consolidated
  **`radix-ui`** package (`import { Slot } from "radix-ui"`), not
  `@radix-ui/react-*`.
- 22 primitives are vendored in `components/ui`: alert, alert-dialog, avatar,
  button, card, checkbox, dialog, dropdown-menu, input, label, pagination,
  popover, select, separator, sheet, sidebar (+ `sidebar-context.tsx`),
  skeleton, sonner, spinner, tabs, tooltip.
- **Customize in place** — these files are ours. House customizations to
  preserve when regenerating: the Button/Tabs variants (§1.6), `motion-reduce:`
  escapes appended to every animated primitive, `skeleton.tsx`/`spinner.tsx`
  consuming the shared motion constants, and `sonner.tsx` (§3.6).
- **No new dependencies** without explicit approval (project rule). Before
  adding a primitive, check whether an existing one + a constant covers it.

### 2.3 The styling hub: `web/src/lib/constants.ts`

Cross-component class strings are exported, documented constants — the de
facto token layer above Tailwind. Key families:

- `CARD_SURFACE_CLASS` + `CARD_INTERACTIVE_SURFACE_CLASS` — the media-card
  chrome: `group relative overflow-hidden rounded-xl border bg-card`, hover
  lift + primary border + glacier glow, property-scoped 200ms transition,
  motion-reduce fallbacks.
- `CARD_MEDIA_HOVER_CLASS` (poster zoom), `CARD_OVERLAY_REVEAL_CLASS` /
  `CARD_ACTION_REVEAL_CLASS` (hover/focus-within overlay + scaled action
  reveal).
- `MOTION_*` — page/section/media-overlay/player-chrome/track/settings
  enter-exit classes and loading/spinner states (§1.5).
- `FOCUS_VISIBLE_RING_CLASS` (§1.7).
- `LIBRARY_TABS_LIST_CLASS` / `LIBRARY_TAB_TRIGGER_CLASS`,
  `TRACK_LIST_CONTAINER_CLASS`, playback-settings select classes, virtual-list
  row heights (`VIRTUAL_LIST_*`).

**Promotion rule**: a class string used by ≥2 components, or containing
motion/focus behavior, moves here (where the contracts tests can see it)
rather than being copy-pasted.

### 2.4 Theme system

Four sync points define the theme; they are intentionally duplicated and
pinned to each other by `theme-drift.test.ts`:

1. `src/assets/styles.css` — the OKLCH tokens (+ hex comments), `:root` and
   `.dark`.
2. `src/assets/boot.css` — pre-hydration paint: themed `html` colors, the
   canvas gradients (a glacier radial tint over a vertical wash), the body
   font stack, and the `#initial-splash` full-screen splash (fades out when
   `AppBoot` sets `data-app-ready="true"`).
3. `index.html` — the inline anti-flash IIFE that reads
   `localStorage["igloo-theme"]` and applies the `dark` class + the
   `<meta name="theme-color">` before first paint (defaults dark, including
   on storage errors).
4. `src/lib/theme.ts` — the runtime API and source of truth for the pinned
   hexes: `THEME_COLORS` / `THEME_TEXT_COLORS`, `getStoredTheme()` (defaults
   `"dark"`), `applyTheme()` (class + meta + listeners), `setTheme()`
   (persists), `subscribeTheme()` for `useSyncExternalStore` consumers
   (`ThemeToggle`, `sonner.tsx`).

If you touch any of the four, run the web tests — the drift test fails on any
mismatch. Live-verified: toggling updates class, storage, and meta correctly,
and a hard reload shows no theme flash.

---

## 3. UI patterns

### 3.1 App shell

`AppShell.tsx`: skip link → shadcn `SidebarProvider` + `AppSidebar`
(`collapsible="icon"`; 16rem expanded / 3rem icon rail / 18rem mobile sheet
that auto-closes on nav) + `SidebarInset` content column.

- Sidebar: logo tile + "Igloo" wordmark linking home; Home / Movies / TV Shows
  / Music / Photos / Settings with lucide icons (active =
  `bg-sidebar-accent` + `text-primary` icon); footer Logout. `SidebarRail`
  gives a click-to-collapse handle.
- Header: sticky `h-14 bg-background/95 backdrop-blur`, border-b. Mobile-only
  `SidebarTrigger`, a `role="search"` form (submits to
  `/search?q=…&tab=all&page=1`), `NotificationBell`, `ThemeToggle`. The search
  input uses the light "frosted" treatment from `lib/input-styles.ts` (an
  allowlisted raw-color exception, §1.2).
- Content: `px-4 py-6 sm:px-6 lg:px-8`; when the mini audio player is
  visible the shell reserves `pb-28 sm:pb-24` so content never hides behind
  it. **Scrolling happens on the window** — virtual lists must use
  `useWindowVirtualizer` (§3.4), never a nested scroll container.

### 3.2 Media cards

One anatomy, shared by `MovieCard`, `InTheatersCard`, `AlbumCard`,
`MusicianCard`, `PlaylistCard`, `WatchRoomCard`:

```
<article class={cn(CARD_SURFACE_CLASS, CARD_INTERACTIVE_SURFACE_CLASS)}>
  <Link aria-label="Uncut Gems 2019" …>            ← whole-card link, full label
    <div class="aspect-2/3 bg-muted">              ← fixed-ratio box (no CLS)
      <img loading="lazy" decoding="async" fetchPriority="low"
           width={500} height={750} class="size-full object-cover" … />
      … onError → centered muted lucide icon (usePosterFallback)
    </div>
    <div class="… bg-linear-to-t from-black/90 via-black/50 to-transparent">
      <h3 class="line-clamp-2 text-sm font-semibold text-white">…</h3>
    </div>
  </Link>
  overlay: opacity-0 group-hover:opacity-100 group-focus-within:opacity-100
           bg-black/30 + centered rounded-full bg-primary Play link/button
</article>
```

Rules: 2:3 posters, square covers, circular musician thumbs; titles clamp at
2 lines; the hover overlay must also reveal on `group-focus-within`; cards
prefetch their detail query on `onMouseEnter`/`onFocus`
(`queryClient.prefetchQuery`); rating badges tier on `bg-aurora` (≥7) /
`bg-aurora/80–90` (≥5) / `bg-muted`.

Grids: the canonical poster grid is
`grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`; home
sections use auto-fit grids
(`grid-cols-[repeat(auto-fit,minmax(min(7.5rem,100%),1fr))]`) inside the
shared `HomeMediaSection` wrapper (heading + count pill + announced summary +
pending/error/empty/grid states). True horizontal rails (cast, chapters,
extras) are `-mx-4 flex overflow-x-auto px-4` with thin glacier scrollbars.

### 3.3 Images

- **Movie/TMDB images are same-origin proxied**: build URLs with
  `buildTmdbImageUrl(path, size)` (`lib/tmdb-image-url.ts`) →
  `/api/tmdb/images/{size}{path}`; sizes are the exported constants
  (`w500` posters, `w1280` backdrops, `w185` profiles, `w92` logos). Music
  images go through `getMediaImageUrl()` (`lib/media-image-url.ts`). Never
  hit `image.tmdb.org` directly from the client.
- **Standard `<img>` recipe**: `loading="lazy" decoding="async"
  fetchPriority="low"`, explicit `width`/`height`, `object-cover`, inside an
  aspect-ratio `bg-muted` box. Add responsive `sizes` on dense grids
  (see `AlbumCard`). Only the login backdrop uses
  `loading="eager"`/`fetchPriority="high"`.
- **Fallbacks, never broken images**: `onError` swaps to a centered muted
  lucide icon (Film / Disc / User) via the `usePosterFallback` hook (tracks
  the failed URL so a changed URL retries). Verified live on covers with
  missing art.
- **Alt policy**: decorative images (a poster inside a link that already has
  an `aria-label`, backdrops) get `alt=""`/`aria-hidden`; informative images
  get descriptive alt (`Album cover for {title}`, `Photo of {name}`).

### 3.4 UI states — loading, empty, error

- **Loading, two tiers.** Route navigations suspend in loaders
  (`ensureQueryData` + `defaultPreload: "intent"`) with one app-wide pending
  screen (`RouterPending` → `AppLoadingScreen`, `role="status"`). Within a
  page, each query renders a **skeleton that matches the real layout's grid
  geometry exactly** (same columns, same aspect boxes) so content arrival
  causes no layout shift — see `AllMoviesTabSkeleton`, `MovieDetailsSkeleton`.
  Primitives: `ui/skeleton.tsx`, `ui/spinner.tsx` (`role="status"`), both on
  the shared `MOTION_LOADING/SPINNER_STATE_CLASS` (pulse/spin +
  `motion-reduce:animate-none`). Skeletons hide their visuals with
  `aria-hidden` under a single `role="status"` + `sr-only` label.
- **Empty, two variants.** Minimal: centered `text-muted-foreground` with a
  large faded lucide icon and one sentence (search no-results, empty tabs).
  Rich CTA: the shared `EmptyState.tsx` — gradient icon orb
  (`size-20 rounded-full bg-linear-to-br from-muted via-muted to-primary/30`),
  title, description, optional pill CTA, optional `bordered` wrapper. Empty
  states announce via `LiveAnnouncer`.
- **Error.** Detection is uniform: `isError || isApiFailure(data)` (the API
  envelope `{ error, message, data }`). Inline query errors use
  `MoviesLoadError` (`role="alert"`, `border-destructive/25 bg-destructive/10
  text-destructive`, "Try again" → `refetch()`); missing resources use
  `MediaNotFound` (destructive `Alert`); mutation failures **toast** via
  `toast-helpers.ts` instead of rendering inline. Placeholder sections
  (`ComingSoon`) reuse the EmptyState language.

### 3.5 Playback surfaces

Playback code is high-risk (see `docs/ffmpeg.md`) — style changes here still
require the full playback test pass.

- **Video player** (`routes/_auth/movies/$id/play.tsx` + `VideoPlayer.tsx` +
  `MoviePlayerControls.tsx`): renders **in-shell** as a windowed player
  (header bar: film icon + title + Back; controls footer below the video —
  progress group, time readouts in `tabular-nums`, rewind / play-pause /
  fast-forward cluster, quality chip, chapter menu, volume, fullscreen).
  In immersive/fullscreen mode (`isImmersiveViewport` /
  `chromeFullscreenMode`) the container goes `fixed inset-0 z-50`, chrome
  becomes absolute overlay panels (`MOTION_PLAYER_CHROME_PANEL_CLASS`,
  `bg-background/95 backdrop-blur-lg`) and **auto-hides on idle**
  (`useIdleControls`), sliding back on pointer/touch/key input. A `sr-only`
  paragraph documents the keyboard map (Space/K, J/L, arrows, M, F, Esc);
  `ResumeDialog` offers resume vs. start-over; announcements via two
  `LiveAnnouncer`s.
- **Audio player** (`AudioPlayer.tsx`, app-wide via `AudioPlayerContext`):
  playing a track opens the **fullscreen Now Playing** view
  (`DialogFullscreenContent`, gradient `from-background via-muted
  to-background`, large art with disc-icon fallback, transport + volume +
  "Track N of M"); "Minimize player (Escape)" collapses it to the **docked
  mini bar** (`fixed inset-x-0 bottom-0 z-40 bg-background/95 backdrop-blur`)
  with track info, transport, close, and a bottom progress strip. The bar
  persists across navigation; the current track's row in lists is
  highlighted (`text-primary` title + tinted row + pause state). Sliders are
  native ranges (§1.7).

### 3.6 Toasts & notifications

- **Sonner wrapper** (`ui/sonner.tsx`): theme-synced via
  `useSyncExternalStore(subscribeTheme, getStoredTheme)`; `richColors`,
  `closeButton`, top-right; toast surfaces tokenized with `!` overrides
  (`!bg-card`/`!bg-muted`, `!border-success/50`, `!border-destructive/50`).
  Fire success/failure through `lib/toast-helpers.ts`
  (`showActionFailed(...)` etc.), not ad-hoc `toast()` calls.
- **NotificationBell** (header): ghost icon button with a glacier unread
  badge pill ("99+" cap; count also in the `aria-label`), opening a `w-80
  bg-card` popover — header row with "Mark all read", `max-h-96` scroll body
  with spinner / "You're all caught up." / `divide-y` list states. Unread
  rows tint `bg-muted/40` with a glacier dot; rows show type label, message,
  relative time, per-row dismiss. Unread count polls every 30s; the list
  query is `enabled` only while open.

### 3.7 Forms & inputs

- Settings pages are shadcn Cards (`border-border/50 bg-muted/30` recipe)
  with labeled inputs, helper text below (`text-muted-foreground text-xs`),
  and per-field or per-card Save buttons; destructive areas get the tinted
  "Danger Zone" card (`border-destructive/*` + destructive text + outline
  destructive actions).
- The header search + login inputs use the frosted light treatment from
  `lib/input-styles.ts` — the sanctioned raw-color exception (§1.2); its
  contract is pinned by `input-styles.test.ts`.
- File inputs, checkboxes, selects: use the vendored primitives; selects in
  dialogs rely on the `data-slot` contract (§1.6).
- Form-level failures toast; field-level validation renders inline with
  `aria-invalid` (styled by the Button/Input base classes).

---

## 4. Audit findings (2026-07-10)

Combined static audit + live inspection (real library: 402 movies, 211
albums / 2,267 tracks; Chrome, both themes, 1440/768/390 viewports). Console
was clean on every page visited. P1 = user-visible/a11y, P2 = maintainability
hazard, P3 = polish.

| # | P | Finding | Evidence / status |
|---|---|---------|-------------------|
| 1 | P2 | **Inter is named but never loaded** — no `@font-face`/package/link; the app silently renders in the OS system font. Fine visually today, but the stack advertises a font users never get, and typography differs per-OS. | `boot.css` body stack; live: `document.fonts` empty, zero font network requests. OPEN |
| 2 | P2 | **Four-way theme duplication** (styles.css / boot.css / index.html / theme.ts). Mitigated by `theme-drift.test.ts`, but there is still no single machine-readable token source; the old dangling `docs/igloo-theme.ts` comment in `styles.css` was removed in this audit (now cites §1.2/§2.4). | §2.4. MITIGATED / OPEN (item was worse before; see §5.2) |
| 3 | P2 | **ESLint raw-color ban only covers `slate/amber/red/emerald`** — `blue-*`, `zinc-*`, `sky-*`, etc. would slip through. (Deliberate `green-*` Spotify exception exists.) | `web/eslint.config.js` `no-restricted-syntax`. OPEN |
| 4 | P2 | **Settings card titles are not headings** — settings pages expose only the `h1`; "Profile Information", "Quick Connect", "Danger Zone" are divs (shadcn `CardTitle`), so screen-reader users can't navigate by heading. | Live: `document.querySelectorAll('h2,h3,h4')` → 0 on `/settings/account`. OPEN |
| 5 | P3 | **Two coexisting focus-ring styles** — `FOCUS_VISIBLE_RING_CLASS` (`ring-2` + offset) vs shadcn base (`ring-[3px] ring-ring/50`). Same color, different weight/offset; visually acceptable live, but one recipe should win eventually. | §1.7. OPEN |
| 6 | P3 | **No `badge.tsx` primitive** — count pills, rating badges, "Under Development" chips are hand-rolled `rounded-full` spans; the aurora rating-tier logic repeats in `InTheatersCard` and `MovieDetailsMetadataChips`. | OPEN |
| 7 | P3 | **Spacing/typography not tokenized** — utility literals only (radius is the lone non-color token). Consistent in practice (§1.4 rhythm), so low urgency; document-and-enforce-by-review. | OPEN (accepted for now) |
| 8 | P1 | **Light-theme `text-success` on `background` ≈ 3.5:1** — below AA for small text. Not covered by `contrast.test.ts` (which checks `success-foreground` *on* `success`, not `text-success` on canvas). | Known caveat carried from the 2026-07-02 round. OPEN |
| 9 | P3 | **`MediaNotFound` is terse and dead-ends** — "404 - The resource you requested was not found." with no way back. | Live on `/movies/999999`. OPEN |
| 10 | P3 | **Stale `scrollbar-*` lint comment** — the eslint ignore says "for the tailwind-scrollbar plugin", but the classes are Tailwind v4 core utilities now (verified live: `scrollbar-width: thin` + glacier thumb are applied). The allowlist may be removable entirely. | `web/eslint.config.js`; live computed styles on the cast rail. OPEN (comment-only) |
| 11 | P3 | **Mobile hero-actions wrap** — on 390px movie detail, Play/Watch/Like fit one row and the "More" (⋮) button wraps alone to a centered second row. Functional, slightly unbalanced. | Live screenshot, `/movies/401` @390. OPEN |
| 12 | — | **Verified strengths (don't churn)**: token discipline in `ui/` (no raw palette hits), airtight `motion-reduce` coverage, hover/focus parity on cards, skip links, anti-flash boot, image fallbacks, TMDB proxying, window-scrolled virtualization (26 rendered rows of 2,267), no horizontal overflow at 390px anywhere visited, clean console. | — |

Items fixed in earlier rounds (toaster tokens, raw red/emerald sweep,
`*-foreground` accent tokens, theme-drift test, poster fallbacks) remain in
place — re-verified during this audit.

## 5. Suggestions & improvements

Prioritized; each maps to a §4 finding.

1. **Fix light-theme `text-success` (§4.8).** Either darken light `--success`
   (needs re-check against `success-foreground` on `success`), or add a
   `--success-text` token for on-canvas text, or restrict `text-success` to
   large text/icons. Extend `contrast.test.ts` to pin `text-success`-on-
   `background` so it can't regress.
2. **Decide the Inter story (§4.1).** Either self-host Inter (`@fontface` +
   woff2 asset — no CDN, keeps the no-external-requests property) or delete
   `Inter,` from the boot stack and own being a system-font app. Update §1.3
   after.
3. **Single token source (§4.2).** Generate the four sync points from one
   typed module (e.g. `src/lib/theme-tokens.ts` with OKLCH + hex per token) —
   or accept the drift-test status quo; if accepted, close the item and note
   it here.
4. **Heading semantics in Settings (§4.4).** Render shadcn `CardTitle` as an
   `h2` (it accepts `asChild`/element overrides) on settings and other
   card-sectioned pages. Cheap, real a11y win.
5. **Widen the ESLint color ban (§4.3).** Extend the regex to all Tailwind
   palette families (keep the documented Spotify-green and input-styles
   exceptions).
6. **Add a `Badge` primitive (§4.6)** with `default/aurora/muted/outline`
   variants and fold the rating-tier logic into one helper.
7. **Unify the focus ring (§4.5).** Pick the shadcn `ring-[3px]/50` recipe or
   the `ring-2`+offset recipe and migrate the other; update §1.7 and the
   `FOCUS_VISIBLE_RING_CLASS` docstring.
8. **Give `MediaNotFound` a way home (§4.9)**: a "Back to Movies/Music"
   button under the alert.
9. **Housekeeping (§4.10, §4.11)**: fix the stale scrollbar lint comment (or
   drop the allowlist if `no-unknown-classes` now recognizes core scrollbar
   utilities), and let the mobile hero-actions row use the full width (e.g.
   `flex-1` on the three main buttons with ⋮ inline).
