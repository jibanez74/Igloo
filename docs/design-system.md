# Igloo Web Design System — UI Development Guideline

This document is the development guideline for the Igloo web client (`web/`). It
describes the visual system as implemented, the conventions every new UI change
must follow, and the July 2026 audit findings (§4) with the fixes that were
applied (§5) — all resolved 2026-07-10 except the deliberately accepted §4
item 7.

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
| Token contrast: body text ≥ 7:1 (AAA), all other fg/surface pairs ≥ 4.5:1 (AA), plus `text-success` on `background`/`card`, both themes | `web/src/test/contrast.test.ts` |
| Generated theme blocks in `styles.css` / `boot.css` / `index.html` match `src/lib/theme-tokens.ts`; every token's OKLCH↔hex pair round-trips | `web/src/test/theme-drift.test.ts` |
| Every shared motion constant carries a `motion-reduce:` escape; every `src/` file with an inline transition/animation has the matching `motion-reduce:` escape | `web/src/test/motion-contracts.test.ts` |
| No raw Tailwind palette classes (all 22 color families) | ESLint `no-restricted-syntax` (error) in `web/eslint.config.js` |
| Class order, duplicates, unknown/conflicting classes | `eslint-plugin-better-tailwindcss` |
| Input styling contracts | `web/src/test/input-styles.test.ts` |
| Shared class-string constants keep their contracts, incl. the single focus-ring recipe | `web/src/test/constants-contracts.test.ts` |

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
  `bg-card`, `text-muted-foreground`, `ring-ring`. The token rules are
  **generated** from `src/lib/theme-tokens.ts` (§2.4).
- **`tw-animate-css`** for enter/exit animations, with a strict
  `motion-reduce:` discipline (§1.5).
- **lucide-react** for all icons.

Theme switching toggles the `dark` class on `<html>` (`web/src/lib/theme.ts`),
persisted under the `igloo-theme` localStorage key, with an anti-flash inline
script in `index.html` (§2.4). Default is dark whenever no stored value exists.

### 1.2 Color tokens

Colors are authored in **OKLCH** in `src/lib/theme-tokens.ts` (value + hex +
comment per token) and rendered into `styles.css` by `bun run generate:theme`
(§2.4) — edit the module, not the CSS. Tokens are paired (`X` surface +
`X-foreground` text) so contrast is guaranteed; every pair is verified in both
themes by `contrast.test.ts`.

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
| `success` / `-foreground` | `#34D399` / `#08131F` | `#167050` / `#FFFFFF` | Success state |
| `accent-teal` / `-foreground` | `#2DD4BF` / `#08131F` | `#0D9488` / `#08131F` | Secondary accent |
| `sidebar` (+ its own fg/primary/accent/border/ring set) | `#0F1A2E` | `#E8F1FA` | Sidebar chrome |
| `chart-1..5` | glacier / teal / aurora / success / danger | same families | Data viz |

Rules:

- **Use semantic tokens, never raw palette classes.** `bg-red-500`,
  `text-slate-400`, `bg-emerald-900` are banned at ESLint error level. The
  sanctioned exceptions are `src/lib/input-styles.ts` and
  `src/routes/login.lazy.tsx` (the intentionally light "frosted glass" input
  treatment that stays light-on-dark in both themes, plus the login backdrop's
  theme-aware photo scrim and frosted-card edge — see the over-media note below).
- `aurora` is identical in both themes and is deliberately **sparing**: rating
  badges and rare highlights, not a general accent.
- Alpha is applied at the call site with the `/NN` modifier
  (`bg-primary/90`, `ring-ring/50`, `border-destructive/25`).
- **Over-media colors are an intentional exception.** Text and overlays that
  sit on posters/backdrops use literal black/white — `bg-black/30`–`/40` dim
  overlays, `bg-linear-to-t from-black/90` scrims, `text-white` titles,
  `shadow-black/30`. These must not track the theme (a poster doesn't change
  with the theme); keep them literal, don't hunt for tokens. The one exception
  is the **login backdrop**, which swaps between a bright (`login-bg-light.webp`)
  and dark (`login-bg-dark.webp`) photo by theme, so its frost/darkening scrim
  gradient and frosted-card border in `login.lazy.tsx` are deliberately
  `dark:`-variant, literal white/slate in light and `background`-token in dark.
- One deliberate brand exception exists: the `SPOTIFY_BRAND_*` constants in
  `lib/constants.ts` (Spotify green, documented there, carried past the lint
  ban with inline `eslint-disable` comments).
- `text-success` is AA-safe on `background` and `card` in both themes
  (light `--success` is a deep glacier green, `#167050`); the pairs are pinned
  by `contrast.test.ts`.

### 1.3 Typography

- **Font stack**: `Inter, ui-sans-serif, system-ui, -apple-system,
  BlinkMacSystemFont, "Segoe UI", sans-serif`, declared once on `body` in
  `web/src/assets/boot.css` (with antialiasing). **Inter is self-hosted**: the
  variable font (`web/public/fonts/InterVariable.woff2`, weights 100–900) is
  loaded by an `@font-face` in `boot.css` with `font-display: swap` and
  preloaded in `index.html` — no CDN, no npm package. Do not declare other
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
  the class constants. Easing is `ease-out` (exits `ease-in`). Applied to
  color changes: control/text hovers use 150ms (`MOTION_MICRO_COLORS_CLASS`);
  whole-card surface tints use 200ms (`MOTION_SETTINGS_SURFACE_CLASS`).
  Applied to floating surfaces: modal surfaces (dialog, sheet, alert-dialog)
  enter/exit at 200ms standard; anchored transient popups (popover, dropdown,
  select, tooltip) at 150ms micro — this split is deliberate, don't unify it.
- **Transitions enumerate properties** — e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]` —
  never `transition-all`.
- **Enter/exit animations** come from `tw-animate-css`:
  `animate-in fade-in slide-in-from-bottom-2 fill-mode-both` for page/section
  entrances (`MOTION_PAGE_ENTER_CLASS`, `MOTION_SECTION_ENTER_CLASS`, and the
  `delay-75` staggered `MOTION_SECTION_ENTER_DELAYED_CLASS`), `animate-out` +
  `fade-out-0` for exits. There are no custom `@keyframes` in `styles.css`.
- **The motion-reduce contract (hard rule)**: every animation/transition ships
  a `motion-reduce:` escape — `motion-reduce:transition-none`,
  `motion-reduce:animate-none`, plus end-state resets
  (`motion-reduce:opacity-100 motion-reduce:translate-y-0
  motion-reduce:scale-100`) so reduced-motion users see the final state, not a
  broken mid-state. Guarded by `motion-contracts.test.ts`, which also scans
  every source file under `src/` (excluding tests and generated files) for
  unescaped transitions and animations.
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
  plain `cn(...)` — `badge.tsx` (a plain variant record:
  `default`/`aurora`/`muted`/`outline`) is the model. Don't introduce cva into
  new components; either add a variant to an existing cva component or export
  a class-string constant (§2.3).
- **`Badge`** (`ui/badge.tsx`) is the primitive for static pills — count
  pills, rating chips, status tags. It is non-interactive by design (a
  `span`); actionable chips are Buttons. Rating-tier colors come from the
  shared helpers in `lib/rating.ts` (§3.2). Pills that live in a semantic
  list nest the Badge inside the `<li>` (movie metadata chips, album
  metadata/genre pills). Navigational pills (e.g. the album page's artist
  pill) are `Link`s composing the pill classes with
  `FOCUS_VISIBLE_RING_CLASS` — deliberately not Badge, which stays
  non-interactive.
- **When to add a variant vs. a constant**: a new *look* for an existing
  primitive (e.g. another Button treatment) → add a cva variant next to its
  siblings. A *cross-component* treatment (card chrome, motion, focus) → an
  exported constant in `lib/constants.ts`.
- All primitives stamp `data-slot="..."`; tests and cross-component behavior
  rely on these (e.g. `SELECT_CONTENT_SLOT_SELECTOR` keeps dialogs open while
  interacting with portaled selects). Keep them when customizing.

### 1.7 Accessibility — non-negotiable

- **One focus recipe.** `--ring` is glacier in both themes, and there is one
  ring recipe: the shadcn `focus-visible:ring-[3px] ring-ring/50 border-ring`
  style, shipped by the primitives and exported for inline (non-shadcn)
  controls as `FOCUS_VISIBLE_RING_CLASS` in `constants.ts` (pinned by
  `constants-contracts.test.ts`). Media cards keep `focus-within:ring-2` on
  the `<article>` (`CARD_FOCUS_WITHIN_RING_CLASS` in `constants.ts`) — the
  intentional whole-card variant — so the card shows focus wherever it lands
  inside. When suppressing the browser outline in
  favor of a ring, always use `outline-hidden`, never `outline-none`: rings
  are box-shadows, which forced-colors mode strips, and `outline-hidden`
  keeps a transparent outline the OS makes visible there.
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
  `scrollbar-thumb-primary/50`) on horizontal media rails; the lint plugin
  recognizes them natively (no allowlist entry needed).
- `eslint-plugin-better-tailwindcss` enforces class order, duplicates, and
  unknown classes; write class lists in its canonical order (the lint
  autofixes).

### 2.2 shadcn/ui usage

- Style **"new-york"**, `baseColor: slate`, CSS variables on, lucide icons
  (`web/components.json`). Components import from the consolidated
  **`radix-ui`** package (`import { Slot } from "radix-ui"`), not
  `@radix-ui/react-*`.
- 23 primitives are vendored in `components/ui`: alert, alert-dialog, avatar,
  badge, button, card, checkbox, dialog, dropdown-menu, input, label,
  pagination, popover, select, separator, sheet, sidebar
  (+ `sidebar-context.tsx`), skeleton, sonner, spinner, tabs, tooltip.
- **Customize in place** — these files are ours. House customizations to
  preserve when regenerating: the Button/Tabs variants (§1.6), the
  house-written `badge.tsx` (plain variant record, no cva), `CardTitle`'s
  `asChild` heading support (§3.7), `motion-reduce:` escapes appended to every
  animated primitive, `skeleton.tsx`/`spinner.tsx` consuming the shared motion
  constants, and `sonner.tsx` (§3.6).
- **No new dependencies** without explicit approval (project rule). Before
  adding a primitive, check whether an existing one + a constant covers it.

### 2.3 The styling hub: `web/src/lib/constants.ts`

Cross-component class strings are exported, documented constants — the de
facto token layer above Tailwind. Key families:

- `CARD_SURFACE_CLASS` — the media-card chrome: `group relative
  overflow-hidden rounded-xl border bg-card`, hover lift + primary border +
  glacier glow, with the property-scoped 200ms transition and motion-reduce
  fallbacks embedded (via `CARD_INTERACTIVE_SURFACE_CLASS`, which stays
  exported for bespoke card shells that bring their own hover styles).
- `CARD_MEDIA_HOVER_CLASS` (poster zoom), `CARD_OVERLAY_REVEAL_CLASS` /
  `CARD_ACTION_REVEAL_CLASS` (hover/focus-within overlay + scaled action
  reveal).
- `MOTION_*` — page/section/media-overlay/player-chrome/track/settings
  enter-exit classes and loading/spinner states (§1.5).
- `FOCUS_VISIBLE_RING_CLASS` (§1.7).
- `LIBRARY_TABS_LIST_CLASS` / `LIBRARY_TAB_TRIGGER_CLASS`,
  `TRACK_LIST_CONTAINER_CLASS` (library/search lists) and
  `DETAIL_TRACK_LIST_CONTAINER_CLASS` (the softer glacier-tinted frame shared
  by the album and musician detail pages), playback-settings select classes,
  virtual-list row heights (`VIRTUAL_LIST_*`).

**Promotion rule**: a class string used by ≥2 components, or containing
motion/focus behavior, moves here (where the contracts tests can see it)
rather than being copy-pasted.

### 2.4 Theme system

**`src/lib/theme-tokens.ts` is the single source of truth**: every themed
color as OKLCH + hex + comment (`THEME_TOKENS`), the boot-only colors
(`BOOT_COLORS`), and `THEME_STORAGE_KEY`. Three files carry generated blocks
rendered from it by `scripts/generate-theme.ts` between
`BEGIN/END GENERATED` markers — **edit the module, run
`bun run generate:theme`, never hand-edit inside the markers**
(`--check` exits 1 if anything is stale):

1. `src/assets/styles.css` — the `:root` / `.dark` token rules. (The
   `@theme inline` mapping above them stays hand-written; add a line there
   when adding a token.)
2. `src/assets/boot.css` — pre-hydration paint: themed `html`/`body` colors,
   the canvas gradients (a glacier radial tint over a vertical wash), and the
   splash message colors. The hand-written zone keeps layout, the body font
   stack, the `@font-face`, and the `#initial-splash` structure (fades out
   when `AppBoot` sets `data-app-ready="true"`).
3. `index.html` — the `<meta name="theme-color">` and the inline anti-flash
   IIFE that reads `localStorage["igloo-theme"]` and applies the `dark` class
   before first paint (defaults dark, including on storage errors).

`src/lib/theme.ts` needs no codegen — it imports the module directly and
derives `THEME_COLORS` / `THEME_TEXT_COLORS` from the canvas tokens; it
remains the runtime API: `getStoredTheme()` (defaults `"dark"`),
`applyTheme()` (class + meta + listeners), `setTheme()` (persists),
`subscribeTheme()` for `useSyncExternalStore` consumers (`ThemeToggle`,
`sonner.tsx`).

`theme-drift.test.ts` re-renders each generated block and diffs it against
the file, and checks every token's OKLCH value round-trips to its declared
hex — so hand edits and module typos both fail CI. OKLCH values in the module
carry enough decimals to round-trip exactly through `src/test/color.ts`; keep
that property when editing. Live-verified: toggling updates class, storage,
and meta correctly, and a hard reload shows no theme flash.

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
<article class={CARD_SURFACE_CLASS}>
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
(`queryClient.prefetchQuery`); rating chips tier via the shared
`criticRatingClass`/`audienceRatingClass` helpers in `lib/rating.ts`
(`bg-aurora` ≥7 / `bg-aurora/80` ≥5 / `bg-muted`), rendered with the `Badge`
primitive where no list semantics are needed (§1.6).

Grids: the canonical poster grid is
`grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`; home
sections use the shared auto-fill grid constants (`HOME_POSTER_GRID_CLASS` /
`HOME_ALBUM_GRID_CLASS` in `lib/constants.ts` —
`grid-cols-[repeat(auto-fill,minmax(min(7.5rem,100%),1fr))]`; **auto-fill,
not auto-fit**, so sparse sections keep cards near the track min width
instead of stretching one poster across the content column; pinned by
`constants-contracts.test.ts`) inside the
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
  `MediaNotFound` (destructive `Alert` + a required "Back to
  Movies/Music/Home" outline link so the page never dead-ends); mutation
  failures **toast** via
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
  destructive actions). **Card section titles are real headings**: render
  `CardTitle` with `asChild` wrapping an `<h2>` (login's is the page `<h1>`)
  so card-sectioned pages are navigable by heading.
- The header search + login inputs use the frosted light treatment from
  `lib/input-styles.ts` — the sanctioned raw-color exception (§1.2); its
  contract is pinned by `input-styles.test.ts`.
- File inputs, checkboxes, selects: use the vendored primitives; selects in
  dialogs rely on the `data-slot` contract (§1.6).
- Form-level failures toast; field-level validation renders inline with
  `aria-invalid` (styled by the Button/Input base classes).

---

## 4. Audit findings (2026-07-10) — resolved 2026-07-10

Combined static audit + live inspection (real library: 402 movies, 211
albums / 2,267 tracks; Chrome, both themes, 1440/768/390 viewports). Console
was clean on every page visited. P1 = user-visible/a11y, P2 = maintainability
hazard, P3 = polish. **All items except #7 (deliberately accepted) were fixed
the same day** — resolutions in the table; details in §5.

| # | P | Finding | Status |
|---|---|---------|--------|
| 1 | P2 | **Inter was named but never loaded** — the app silently rendered in the OS system font. | FIXED (2026-07-10): self-hosted `InterVariable.woff2` (§1.3). |
| 2 | P2 | **Four-way theme duplication** (styles.css / boot.css / index.html / theme.ts) with no single machine-readable token source. | FIXED (2026-07-10): `src/lib/theme-tokens.ts` + `bun run generate:theme` (§2.4). |
| 3 | P2 | **ESLint raw-color ban only covered `slate/amber/red/emerald`** — other families would slip through. | FIXED (2026-07-10): ban widened to all 22 families; Spotify green carries inline eslint-disables; the one purple offender migrated to `accent-teal`. |
| 4 | P2 | **Settings card titles were not headings** — screen-reader users couldn't navigate settings by heading. | FIXED (2026-07-10): `CardTitle asChild` + `<h2>` on all card-sectioned pages; login card title is the page `<h1>` (§3.7). |
| 5 | P3 | **Two coexisting focus-ring styles** (`ring-2`+offset constant vs shadcn `ring-[3px]/50` base). | FIXED (2026-07-10): the shadcn recipe won; `FOCUS_VISIBLE_RING_CLASS` now matches the primitives and is pinned (§1.7). |
| 6 | P3 | **No `badge.tsx` primitive**; aurora rating-tier logic duplicated in `InTheatersCard` and `MovieDetailsMetadataChips` (with a `/80` vs `/90` mid-tier drift). | FIXED (2026-07-10): `ui/badge.tsx` + `lib/rating.ts` helpers, adopted for rating badges and the home count pill (§1.6, §3.2). |
| 7 | P3 | **Spacing/typography not tokenized** — utility literals only (radius is the lone non-color token). Consistent in practice (§1.4 rhythm). | OPEN — **accepted**; document-and-enforce-by-review. |
| 8 | P1 | **Light-theme `text-success` on `background` ≈ 3.5:1** — below AA for small text, and unguarded. | FIXED (2026-07-10): light `--success` → `#167050` (5.6:1 canvas / 6.1:1 card / 4.6:1 on `success/15` tints), fg → white; pairs pinned in `contrast.test.ts` (§1.2). |
| 9 | P3 | **`MediaNotFound` was terse and dead-ended.** | FIXED (2026-07-10): required `backTo`/`backLabel` props render an outline back link (§3.4). |
| 10 | P3 | **Stale `scrollbar-*` lint comment** ("tailwind-scrollbar plugin" — the classes are Tailwind v4 core). | FIXED (2026-07-10): the allowlist entry was removable entirely; lint passes without it (§2.1). |
| 11 | P3 | **Mobile hero-actions wrap** — the ⋮ button wrapped alone to a second row at 390px. | FIXED (2026-07-10): `flex-1 sm:flex-none` on Play/Watch/Like (+ `px-3 sm:px-6`) keeps all four controls on one row. |
| 12 | — | **Verified strengths (don't churn)**: token discipline in `ui/` (no raw palette hits), airtight `motion-reduce` coverage, hover/focus parity on cards, skip links, anti-flash boot, image fallbacks, TMDB proxying, window-scrolled virtualization (26 rendered rows of 2,267), no horizontal overflow at 390px anywhere visited, clean console. | — |

Items fixed in earlier rounds (toaster tokens, raw red/emerald sweep,
`*-foreground` accent tokens, theme-drift test, poster fallbacks) remain in
place — re-verified during this audit.

## 5. Suggestions & improvements — applied 2026-07-10

Each item maps to a §4 finding; the chosen resolution is recorded here.

1. **Light-theme `text-success` (§4.8) — done.** Darkened light `--success` to
   `oklch(0.487 0.097 163.5)` `#167050` (a deep glacier green: 5.62:1 on
   canvas, 6.06:1 on card, 4.56:1 as text on `bg-success/15` tints) and
   flipped light `--success-foreground` to white (6.06:1), mirroring the
   light `destructive` pattern. Light `--chart-4` tracks it.
   `contrast.test.ts` now pins `--success` on `--background` and `--card` in
   both themes.
2. **Inter (§4.1) — self-hosted.** `web/public/fonts/InterVariable.woff2`
   (variable, 100–900) + `@font-face` with `font-display: swap` in `boot.css`
   + a `<link rel="preload">` in `index.html`. No CDN, no npm package; the
   font stack was already correct.
3. **Single token source (§4.2) — generated.** `src/lib/theme-tokens.ts` is
   the machine-readable source; `scripts/generate-theme.ts`
   (`bun run generate:theme`, `--check` for CI) renders the marked blocks in
   `styles.css` / `boot.css` / `index.html`; `theme.ts` imports the module
   directly; `theme-drift.test.ts` became a regenerate-and-diff + OKLCH↔hex
   round-trip guard. See §2.4.
4. **Heading semantics in Settings (§4.4) — done.** `CardTitle` gained
   `asChild` (radix `Slot`); all settings cards, `DevicesCard`, and
   `QuickConnectApproveCard` render `<h2>` titles; the old
   `role="heading"` hack in playback settings was removed; the login card
   title became the page `<h1>`.
5. **ESLint color ban (§4.3) — widened** to all 22 Tailwind families (Literal
   + TemplateElement selectors). The Spotify-green constants carry inline
   `eslint-disable` comments; the input-styles/login allowlist block is
   unchanged; the TV Shows library card's purple moved to `accent-teal`.
6. **`Badge` primitive (§4.6) — added** (`ui/badge.tsx`,
   `default/aurora/muted/outline`, plain variant record per the cva policy,
   non-interactive). Rating tiers folded into `lib/rating.ts`
   (`criticRatingClass`/`audienceRatingClass`, mid tier standardized on
   `/80`). Adopted where it consolidates: both rating badges and the
   `HomeMediaSection` count pill. Deliberately left: `ComingSoon` chip
   (`role="status"`, bespoke), `NotificationBell` micro-badge
   (positioning-critical), genre pills (`<li>` semantics).
7. **Focus ring (§4.5) — unified on the shadcn recipe.**
   `FOCUS_VISIBLE_RING_CLASS` =
   `focus-visible:border-ring focus-visible:ring-[3px]
   focus-visible:ring-ring/50 focus-visible:outline-hidden`, pinned by
   `constants-contracts.test.ts`. Card `focus-within:ring-2` stays as the
   whole-card variant.
8. **`MediaNotFound` (§4.9) — done.** Required `backTo` (`"/" | "/movies" |
   "/music"`) + `backLabel` props; all four call sites updated (in-theaters
   details go back to Home, where the rail lives).
9. **Housekeeping (§4.10, §4.11) — done.** The `scrollbar-.*` lint allowlist
   entry was dropped outright (the plugin knows the v4 core utilities), and
   the hero actions row uses `flex-1 sm:flex-none` + tighter mobile padding
   so Play/Watch/Like/⋮ share one row at 390px.
