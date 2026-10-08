# Igloo Web Design System — UI Development Guideline

This document is the development guideline for the Igloo web client (`web/`).
It describes the visual system **as implemented** and the conventions every new
UI change must follow. Where it states a rule, the rule is binding; where the
code disagrees, that is a bug in one of them, to be reconciled rather than
ignored.

Scope: the web client only. Two earlier revisions were removed deliberately and
are recoverable from git history — an Android TV / Jetpack Compose port guide,
and the 2026-07-10 audit log (findings and fixes, all closed).

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
| Token contrast: body text ≥ 7:1 (AAA), all other fg/surface pairs ≥ 4.5:1 (AA), plus `text-success` on `background`/`card`, both themes | `web/src/test/shared/contrast.test.ts` |
| Generated theme blocks in `styles.css` / `boot.css` / `index.html` match `src/lib/theme-tokens.ts`; every token's OKLCH↔hex pair round-trips | `web/src/test/shared/theme-drift.test.ts` |
| Every shared motion constant carries a `motion-reduce:` escape; every `src/` file with an inline transition/animation has the matching `motion-reduce:` escape | `web/src/test/shared/motion-contracts.test.ts` |
| Every focus indicator in `src/` uses a shared focus-ring recipe — no hand-written widths, offsets, or `focus:` (mouse-visible) rings | `web/src/test/shared/focus-contracts.test.ts` |
| No string in `src/` hard-codes a keyboard-shortcut hint suffix (`"Mute (M)"`) — hints go through `useShortcutHints` | `web/src/test/shared/shortcut-hint-contracts.test.ts` |
| Input styling contracts | `web/src/test/shared/input-styles.test.ts` |
| Shared class-string constants keep their contracts | `web/src/test/lib/constants-contracts.test.ts` |
| No raw Tailwind palette classes (all 22 color families) | ESLint `no-restricted-syntax` (**error**) in `web/eslint.config.js` |
| Class order, duplicates, unknown classes | `eslint-plugin-better-tailwindcss` (**warn**; `bun run lint` runs `--max-warnings 0`, so warnings still fail CI) |

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
- **lucide-react** for all icons, with one exception: the **brand mark**.
  `components/app/BrandMark.tsx` draws the igloo from `lib/brand-mark.ts`,
  the same glyph the app icons are generated from (§2.4), in `currentColor`
  with transparent cut-outs so the parent picks the tile: the sidebar shows
  it on a `bg-primary` tile, the login card and loading screen in a
  `bg-muted` orb tinted `text-primary`. Never stand in a lucide glyph or a
  letter for the logo — one mark everywhere, as the streaming apps do.

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
  `src/routes/login.tsx` (the intentionally light "frosted glass" input
  treatment that stays light-on-dark in both themes, plus the login backdrop's
  theme-aware photo scrim and frosted-card edge — see the over-media note
  below). Both are allowlisted by name in `eslint.config.js`; `src/test/**` is
  exempt wholesale so the contract tests can assert on raw shades.
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
  gradient and frosted-card border in `login.tsx` are deliberately
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
- **Spacing and type stay deliberately untokenized.** Both are the standard
  Tailwind scales as utility literals. Tokenizing them was considered and
  rejected: the values are already consistent in practice, and a token layer
  over a scale that every developer already knows buys indirection, not
  safety. Radius is the lone non-color token because it is the one value
  shadcn's primitives read. This rule is enforced by review, not a test.
  Rhythm in practice: `gap-4` in poster grids, `gap-6` between card blocks,
  page sections `mt-6 md:mt-8`, shell content padding
  `px-4 py-6 sm:px-6 lg:px-8`.
- **Aspect ratios are part of the design vocabulary**: `aspect-2/3` movie and
  show posters, `aspect-square` album covers, musician thumbs and playlist
  covers, `aspect-21/9` detail-page backdrops (clamped
  `max-h-[min(42vh,22rem)]` at `md+`; the shared `DetailBackdrop` owns the
  scrims), `aspect-video` trailers, extras, episode stills and the up-next
  thumbnail.
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
  select) at 150ms micro — this split is deliberate, don't unify it.
- **Transitions enumerate properties** — e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]` —
  never `transition-all`.
- **Enter/exit animations** come from `tw-animate-css`. A page entrance both
  fades and lifts (`MOTION_PAGE_ENTER_CLASS`:
  `animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-300`); a
  section entrance only fades (`MOTION_SECTION_ENTER_CLASS` and its `delay-75`
  staggered twin `MOTION_SECTION_ENTER_DELAYED_CLASS`:
  `animate-in fade-in-0 fill-mode-both duration-200`) — the split is
  deliberate, since a page full of independently sliding sections reads as
  jitter. Exits use `animate-out` + `fade-out-0`. There are no custom
  `@keyframes` in `styles.css`.
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
of the system. Its supported variants are `default`, `destructive`, `outline`,
`ghost`, **`accent`** (primary + `shadow-md`), and **`accent-pill`**
(rounded-full primary). Its supported sizes are `default`, `sm`, `lg`, `icon`,
and `icon-sm`. The base string carries the focus ring, disabled opacity,
`aria-invalid` styling, a property-scoped 150ms transition with
`motion-reduce:transition-none`, and stamps `data-variant`/`data-size`.

- **A disabled primary button is grey, not dim glacier.** The primary-fill
  variants (`default`, `accent`, `accent-pill`) swap to `bg-muted
  text-muted-foreground` at full opacity when `disabled`, with no shadow, so
  a "Send Request" that cannot be sent yet never reads as a second live
  action beside the real one. Outline, ghost and destructive keep the base
  half-opacity. Guarded by `test/ui/button.test.tsx`.

- **`accent-pill` stays a pill at every size.** cva emits
  base → variant → size → `className`, and `cn` is `twMerge`, so the last
  conflicting class wins: `sm` and `lg` re-declare `rounded-md`. A
  `compoundVariants` entry re-applies `rounded-full` to `accent-pill` at
  those sizes, so call sites never re-assert it (guarded by
  `test/ui/button.test.tsx`). Other variants still square off at `sm`/`lg`:
  an `outline` button meant to pair with a pill (the album, musician and
  playlist Shuffle buttons) passes `rounded-full` in `className`. Watch for
  this whenever two sibling buttons are meant to match — one picking up a
  `size` is enough to break the pair.
- **Tabs share one look** (`web/src/components/ui/tabs.tsx`): a bordered
  `bg-muted/50` list; the active trigger is a glacier primary-fill pill
  (`data-[state=active]:bg-primary … shadow-primary/20`). Library pages layer
  responsive grid sizing on top via `LIBRARY_TABS_LIST_CLASS` /
  `LIBRARY_TAB_TRIGGER_CLASS` from `constants.ts`. Tab state lives in URL
  search params (validated in `types/route-search.ts`), never local state.
- **cva policy**: `button.tsx` is the only primitive that uses
  `class-variance-authority`. Everything else composes plain `cn(...)` —
  `badge.tsx` (a plain `default`/`outline` variant record) is the model. Don't
  introduce cva into new components; either add a variant to an existing cva
  component or export a class-string constant (§2.3).
- **`Badge`** (`ui/badge.tsx`) is the primitive for static pills — count
  pills, rating chips, status tags. It is non-interactive by design (a
  `span`); actionable chips are Buttons. Rating-tier colors come from the
  shared helpers in `lib/rating.ts` (§3.2). Pills that live in a semantic
  list nest the Badge inside the `<li>` (movie metadata chips, album
  metadata/genre pills). Navigational pills (e.g. the album page's artist
  pill) are `Link`s composing the pill classes with
  `FOCUS_VISIBLE_RING_CLASS` — deliberately not Badge, which stays
  non-interactive.
- **Narrow primitive contracts are intentional.** Alert is destructive-only;
  AlertDialog has one default size; dropdown items are non-inset; Select uses
  its default trigger size and item-aligned content; Separator is horizontal
  and decorative. Add a branch only when a production surface requires it.
- **A dialog that can outgrow the viewport scrolls its body, not itself.**
  Technical Details and Edit Movie compose `DIALOG_SCROLL_CONTENT_CLASS` on
  `DialogContent` (a flex column capped at `100svh - 2rem`, no padding of its
  own), `DIALOG_SCROLL_HEADER_CLASS` on the `DialogHeader`, and
  `DIALOG_SCROLL_BODY_CLASS` on the one element that scrolls. The close
  button is positioned against the non-scrolling box, so it never rides the
  scrollbar track or scrolls out of reach, and a title the dialog focuses on
  open carries `outline-hidden` with no ring (§1.7).
- **When to add a variant vs. a constant**: a new *look* for an existing
  primitive (e.g. another Button treatment) → add a cva variant next to its
  siblings. A *cross-component* treatment (card chrome, motion, focus) → an
  exported constant in `lib/constants.ts`.
- All primitives stamp `data-slot="..."`; tests and cross-component behavior
  rely on these (e.g. `SELECT_CONTENT_SLOT_SELECTOR` keeps dialogs open while
  interacting with portaled selects). Keep them when customizing.

### 1.7 Accessibility — non-negotiable

- **One focus recipe, in three prefixes.** `--ring` is glacier in both themes,
  and every focus indicator in the app is the shadcn
  `focus-visible:ring-[3px] ring-ring/50 border-ring` style. It reaches the
  page three ways, all exported from `constants.ts`:
  - `FOCUS_VISIBLE_RING_CLASS` — the default, for any inline (non-shadcn)
    control. The vendored primitives ship the same string in their own base
    classes.
  - `CARD_FOCUS_WITHIN_RING_CLASS` — the whole-card variant, on a media card's
    `<article>`, so the card shows focus wherever it lands inside.
  - `PEER_FOCUS_VISIBLE_RING_CLASS` — for a rich `<Label>` standing in for an
    `sr-only` radio (the TMDB and Spotify pickers), where the ring must follow
    the peer input's focus rather than the label's.

  Never hand-write a ring. `focus-contracts.test.ts` parses every file under
  `src/` and fails on a literal declaring its own ring width, a
  `ring-offset-*`, or a `focus:` (rather than `focus-visible:`) prefix — a
  `focus:` ring shows on mouse click, which is noise for pointer users. The
  only allowlisted files are `lib/constants.ts`, which declares the recipes
  themselves, and `ui/sidebar.tsx`, which rings on the sidebar's own
  `--sidebar-ring` token set.

  When suppressing the browser outline in favor of a ring, always use
  `outline-hidden`, never `outline-none`: rings are box-shadows, which
  forced-colors mode strips, and `outline-hidden` keeps a transparent outline
  the OS makes visible there.
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
  regions so repeated messages re-announce) for async state changes; a section
  announces its empty state. A *loaded* summary is not announced — it is an
  `sr-only` paragraph tied to the section through `aria-describedby`, so it
  is there on demand rather than interrupting a reader four times as a page's
  rows resolve, and it stays out of sight because the count pill, the error
  alert and the empty state already carry the same words where the eye lands.
  Errors announce themselves through `role="alert"` (§3.4).
- **Skip links**: a global "Skip to page content" in `AppShell` targeting
  `#main`, plus per-page section skip navs on long pages. Every detail page —
  movie, show, in-theaters, album, musician — uses the one shared
  `DetailSkipLinks` (`sr-only focus-within:not-sr-only`, links on
  `SKIP_LINK_CLASS`); it takes the page title anchor plus a `sections` list
  where a section that is not rendered passes `false`, so the list reads
  exactly what is on the page. Do not write a page-specific variant.
  Their targets are the page `h1` and the section headings (`tabIndex={-1}`),
  which carry the focus ring via `DETAIL_SECTION_HEADING_CLASS` /
  `DETAIL_RAIL_HEADING_CLASS` (or compose `FOCUS_VISIBLE_RING_CLASS` directly,
  where a hero `h1` keeps its own type scale) so a keyboard user sees where a
  skip link landed.
- **A skip-link target rings; a programmatic focus move does not.** The
  distinction is who moved focus. A skip link is a deliberate keyboard
  navigation, so its landing must be visible — a `tabIndex={-1}` target with a
  bare `outline-hidden` and no ring is a bug, and one no test catches
  (`outline-hidden` alone is not a ring declaration, so
  `focus-contracts.test.ts` sees nothing). Focus that the *app* moves so a
  screen reader announces a change the user did not navigate to —
  `QuickConnectApproveCard`'s wizard-step headings, `AppShell`'s `#main` — is
  correctly left un-ringed: a ring on a non-interactive element the user never
  focused is visual noise. Don't "fix" those.
- **Labels everywhere**: icon-only buttons get `aria-label`; decorative
  icons/images get `aria-hidden="true"`/`alt=""`; cards carry a full
  `aria-label` ("Play Uncut Gems 2019"); toggles use `aria-pressed`; nav uses
  `aria-current`; sections use `role="region" aria-labelledby`.
- **`aria-label` only where the role supports it**: `aria-label` names elements
  whose role accepts a name from the author — buttons, links, inputs, `role="list"`,
  regions, dialogs. It does **not** reliably name `<li>` (`role="listitem"`),
  `<time>`, or plain `<span>`/`<div>`; browsers and screen readers inconsistently
  ignore it there. For those, put the spoken text *in the content* as an `sr-only`
  span and mark the visually-formatted value `aria-hidden="true"`
  (the `MovieDetailsMetadataChips` / `ShowDetailsMetadataChips` pattern):

  ```tsx
  <li>
    <Star aria-hidden="true" />
    <span className="sr-only">Critic rating: 8.5 out of 10</span>
    <span aria-hidden="true">8.5</span>
  </li>
  ```

  Never leave a non-interactive element whose only content is `aria-hidden` —
  assistive tech lands on an empty node.
- **Keyboard-shortcut hints follow the input device.** A player control names
  its shortcut as a suffix of its accessible name ("Pause (Space or K)"), and
  a player may add an `sr-only` keyboard map ("Keyboard shortcuts: …") as a
  paragraph or its dialog's description. Both come from `useShortcutHints`:
  `withShortcut(label, keys)` builds the name and `showShortcutHints` gates
  the map. A touch-first device (`(hover: none) and (pointer: coarse)`, via
  `usePrefersCoarsePointer`) gets neither, because a screen reader there would
  read out keys nobody can press; the shortcuts themselves stay bound for an
  attached keyboard. Never hard-code the suffix. Gate on the pointer, not on
  `useIsMobile`: a narrow desktop window still has a keyboard. The playlist
  reorder list (`DraggableTrackList`) follows the same rule: its handles are
  described by dnd-kit's key map only when `showShortcutHints` is true (a
  touch-first device gets the hold-and-drag gesture instead), and its
  pick-up announcement names space and escape only when the drag was started
  from the keyboard.
- **Every route titles the page through its `head`, never in JSX.** The
  document title is the first thing a screen reader announces after a
  navigation, and it names the browser tab and history entry. Routes declare
  TanStack Router's `head` option, and `<HeadContent />` in `__root.tsx`
  renders it through React 19's head hoisting. There are two rules:
  - **Build titles and descriptions with `routeHead()`** (`src/lib/route-head.ts`).
    It produces `<Page> - Igloo` and caps descriptions at 160 characters.
  - **Where the values come from:**
    - *Static* values: a module-level constant (`const MOVIES_HEAD = routeHead(…)`)
      returned from `head: () => MOVIES_HEAD`.
    - *Data-driven* values: the loader returns only the few fields the head needs,
      and `head: ({ loaderData }) => …` builds the strings. It must fall back to a
      generic title when `loaderData` is missing, because a head that throws is
      only logged.
    - *After an edit* that renames what the title shows (a movie identify or
      edit, a playlist rename): await the details refetch, then
      `router.invalidate()` that route, since heads only rerun on a router load.

  The root head supplies the defaults: the title `Igloo` and the app
  description. A child's `title` and same-named `meta`
  override them. A subpage that sets only a title (settings tabs, the players)
  keeps its parent's description. `index.html` keeps only static, non-route
  metadata: a plain pre-boot `<title>`, the `robots: noindex, nofollow` meta
  (seen by crawlers that never run JS), `application-name`,
  `apple-mobile-web-app-title`, the generated `theme-color` and the
  manifest/icon links. Don't put
  `<title>`/`<meta>` in components: a second title or description tag would
  compete with the route's.

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
- `eslint-plugin-better-tailwindcss` checks class order, duplicates, and
  unknown classes. It reports at **warn**, but `bun run lint` runs
  `--max-warnings 0`, so a warning still fails the build; write class lists in
  its canonical order (the lint autofixes). Line wrapping is explicitly
  disabled — Prettier owns that.

### 2.2 shadcn/ui usage

- Style **"new-york"**, `baseColor: slate`, CSS variables on, lucide icons
  (`web/components.json`). Components import from the consolidated
  **`radix-ui`** package (`import { Slot } from "radix-ui"`), not
  `@radix-ui/react-*`.
- 20 primitives are vendored in `components/ui`: alert, alert-dialog, avatar,
  badge, button, card, checkbox, dialog, dropdown-menu, input, label,
  pagination, popover, select, separator, sheet, sidebar, sonner, spinner,
  tabs — plus `sidebar-context.tsx`, which holds the sidebar's context so the
  primitive and `AppSidebar` can both read it.
- Their exported subcomponents are intentionally limited to production use.
  Portal, overlay, and Select scroll helpers stay private to their owning
  primitive. Sheet is the left mobile-navigation surface, and Sidebar exports
  only the provider, shell/inset, trigger/rail, and the header/content/footer,
  group, and menu pieces used by `AppSidebar`.
- **Customize in place** — these files are ours. House customizations to
  preserve when regenerating: the Button/Tabs variants (§1.6), the
  house-written `badge.tsx` (plain variant record, no cva), `CardTitle`'s
  `asChild` heading support (§3.7), `motion-reduce:` escapes appended to every
  animated primitive, `spinner.tsx` consuming the shared motion constants,
  the fixed navigation contracts, and `sonner.tsx` (§3.6).
- **No new dependencies** without explicit approval (project rule). Before
  adding a primitive, check whether an existing one + a constant covers it.

### 2.3 Shared constants: `web/src/lib/constants.ts`

One module holds every shared frontend constant — query keys, protocol
records, page sizes, TMDB image sizes, playback timings — and, in its second
half, the cross-component class strings that act as the de facto token layer
above Tailwind. Those are what this section is about; each carries a comment
explaining what it owns. The styling families:

- **Cards** — `CARD_SURFACE_CLASS`, the media-card chrome: `group relative
  overflow-hidden rounded-xl border border-border bg-card`, hover lift +
  primary border + glacier glow, with the property-scoped 200ms transition and
  motion-reduce fallbacks embedded (via `CARD_INTERACTIVE_SURFACE_CLASS`,
  which stays exported for bespoke card shells that bring their own hover
  styles). Plus `CARD_MEDIA_HOVER_CLASS` (poster zoom),
  `CARD_OVERLAY_REVEAL_CLASS` / `CARD_ACTION_REVEAL_CLASS` (hover/focus-within
  overlay + scaled action reveal), `CARD_FOCUS_WITHIN_RING_CLASS` (§1.7) and
  `OVER_MEDIA_BADGE_CLASS` (the corner chip over a poster).
- **Motion** — `MOTION_*`: page/section, media-overlay and -dialog,
  player-chrome, track-row, settings-surface and decorative classes, plus the
  loading/spinner states and the three `MOTION_DURATION_*_MS` numbers (§1.5).
- **Focus** — the three ring recipes (§1.7). `DETAIL_SECTION_HEADING_CLASS`,
  `DETAIL_RAIL_HEADING_CLASS`, `SKIP_LINK_CLASS`, `PLAYER_ICON_BUTTON_CLASS`
  and `PLAYER_PRIMARY_BUTTON_CLASS` all compose `FOCUS_VISIBLE_RING_CLASS`
  rather than restating it.
- **Page chrome** — `DETAIL_HERO_*` (hero shell, content, and the three
  literal over-media scrims), `LIBRARY_TABS_LIST_CLASS` /
  `LIBRARY_TAB_TRIGGER_CLASS`, `LIBRARY_POSTER_GRID_CLASS` (the fixed-column
  library grid) and `LIBRARY_MENU_ITEM_CLASS` (items in a library page's More
  menu), `HOME_POSTER_GRID_CLASS` / `HOME_ALBUM_GRID_CLASS` (§3.2), `MINI_PLAYER_CLEARANCE_*` (the shell's
  reserved space under the mini bar, §3.1), `SETTINGS_*` (card surface, input,
  select and reset-button chrome for the Settings pages, §3.7).
- **Lists** — `TRACK_LIST_CONTAINER_CLASS` (library/search lists),
  `DETAIL_TRACK_LIST_CONTAINER_CLASS` (the softer glacier-tinted frame shared
  by the album and musician detail pages and the show episode list), and the
  `VIRTUAL_LIST_*` row heights.

**Promotion rule**: a class string used by ≥2 components, or containing
motion/focus behavior, moves here (where the contracts tests can see it)
rather than being copy-pasted. A string used once stays local unless naming it
adds meaning.

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
   when `dismissBootSplash` in `lib/boot-splash.ts` sets
   `data-app-ready="true"`; the same call hides the splash from assistive
   tech and removes it on `transitionend` with an uncancellable watchdog, so
   a remount during the fade can never leave it blocking input).
3. `index.html` — the `<meta name="theme-color">` and the inline anti-flash
   IIFE that reads `localStorage["igloo-theme"]` and applies the `dark` class
   before first paint (defaults dark, including on storage errors).

**App icons are generated too.** `scripts/generate-icons.ts`
(`bun run generate:icons`, `--check`) renders `public/favicon.svg` and
`public/manifest.webmanifest` as whole files from the tokens, the glyph in
`src/lib/brand-mark.ts` and `APP_NAME`/`APP_DESCRIPTION` from
`src/lib/constants.ts` (the root route's head uses the same two), then screenshots the SVG with Playwright's bundled
Chromium into `icon-192.png`, `icon-512.png`, `icon-maskable-512.png` (glyph
inside the 80 % safe zone), `apple-touch-icon.png` (180 px, full-bleed) and a
PNG-in-ICO `favicon.ico` (16 + 32 px). The mark is fixed across themes, like
every streaming app's tile: the dark `--background` canvas with a faint
`--primary` radial glow at the top (the boot.css tint, stronger), and the
igloo in `--primary`. Rerun the script after changing any source and
commit the outputs; `icon-drift.test.ts` diffs the SVG and manifest against
their renders and checks the ICO structure, while the PNG bytes are not
compared because they vary by Chromium version.

`src/lib/theme.ts` needs no codegen — it imports the module directly and
derives `THEME_COLORS` / `THEME_TEXT_COLORS` from the canvas tokens; it
remains the runtime API: `getStoredTheme()` (boot-time persistence read,
defaults `"dark"`), `getActiveTheme()` (the applied in-memory theme),
`applyTheme()` (class + meta + active state + listeners), `setTheme()`
(persists when possible and always applies), and `subscribeTheme()` for
`useSyncExternalStore` consumers (`ThemeToggle`, `sonner.tsx`, and the login
backdrop).

`theme-drift.test.ts` re-renders each generated block and diffs it against
the file, and checks every token's OKLCH value round-trips to its declared
hex — so hand edits and module typos both fail CI. OKLCH values in the module
carry enough decimals to round-trip exactly through
`src/test/helpers/color.ts`; keep
that property when editing. Live-verified: toggling updates class, storage,
and meta correctly, and a hard reload shows no theme flash.

---

## 3. UI patterns

### 3.1 App shell

`AppShell.tsx`: skip link → shadcn `SidebarProvider` + `AppSidebar` (a fixed
left-side, icon-collapsible contract; 16rem expanded / 3rem icon rail / 18rem
mobile sheet that auto-closes on nav) + `SidebarInset` content column. The
provider owns this state internally; the rail and Ctrl/Cmd+B toggle desktop
collapse, while the mobile trigger controls the sheet.

- Sidebar: `BrandMark` on a `bg-primary` tile + "Igloo" wordmark linking
  home; Home / Movies / TV Shows
  / Music / Photos / Settings with lucide icons (active =
  `bg-sidebar-accent` + `text-primary` icon); footer Logout. `SidebarRail`
  gives a click-to-collapse handle.
- Header: sticky `h-14 bg-background/95 backdrop-blur-sm`, border-b.
  Mobile-only `SidebarTrigger`, then `app/Header.tsx` — a `role="search"` form
  (submits to `/search?q=…&tab=all&page=1`), `NotificationBell`,
  `ThemeToggle`. The search input uses the light "frosted" treatment from
  `lib/input-styles.ts` (an allowlisted raw-color exception, §1.2).
- Content: `px-4 py-6 sm:px-6 lg:px-8`; when the mini audio player is visible
  the shell adds `MINI_PLAYER_CLEARANCE_PADDING_CLASS` (`pb-28 sm:pb-24`) so
  content never hides behind it. **Scrolling happens on the window** — virtual
  lists must use `useWindowVirtualizer` (§3.4), never a nested scroll
  container. For the same reason the content column clips with
  `overflow-x-clip`, not `overflow-x-hidden`/`auto`: those create a scroll
  container, and sticky children inside pages then stop sticking to the
  window.

**Unknown URLs** land on the shell's catch-all route (`routes/_auth/$.tsx`):
a "Page not found" heading over `MediaNotFound` (title "Not found", "Back to
Home"), titled through `head` like every page, so a mistyped address keeps
the sidebar, the header and a way back instead of the router's bare text.

**The home page** is the shell's canonical composition: a hero heading, then
six sections in order — `WatchRooms`, `ContinueWatching`, `LatestMovies`,
`LatestShows`, `LatestAlbums`, `MoviesInTheaters`. All but `WatchRooms` render
through the shared `HomeMediaSection` (heading + count pill + `sr-only`
described-by summary + pending/error/empty/grid states), and the route loader `ensureQueryData`s
every one of their queries, so the page arrives complete rather than popping in
section by section. A new home row is a `HomeMediaSection` with one of the
`HOME_*_GRID_CLASS` grids (§3.2) and its query added to the loader.

### 3.2 Media cards and library pages

#### Poster cards

Every 2:3 poster card is `components/shared/PosterCard.tsx`, implemented once.
`MovieCard`, `ShowCard`, `InTheatersCard` and `ContinueWatchingEpisodeCard`
pass it their links, labels, badge and watch progress rather than repeating the
markup; a new poster card belongs there too. Reach for a fresh `<article>` only
when the anatomy genuinely differs — square covers (`AlbumCard`,
`PlaylistCard`, `MoviePlaylistCard`), circular thumbs (`MusicianCard`), or the
horizontal `WatchRoomCard`. All of them wear `CARD_SURFACE_CLASS`.

```
<article class={cn(CARD_SURFACE_CLASS, CARD_FOCUS_WITHIN_RING_CLASS)}>
  <Link aria-label="Uncut Gems 2019" …>       ← whole-card link, full label
    <div class="aspect-2/3 bg-muted">              ← fixed-ratio box (no CLS)
      <img loading="lazy" decoding="async" fetchPriority="low"
           width={500} height={750} class="size-full object-cover" … />
      … onError → centered muted lucide icon (usePosterFallback)
      {playLink && <div class={CARD_OVERLAY_REVEAL_CLASS} … bg-black/30 />}
      {badge}                                      ← optional corner chip
      <div class="h-[60%] bg-linear-to-t from-black via-black/90 via-50% to-transparent" />
      {progress && <WatchProgressBar … />}
    </div>
    <div class="absolute inset-x-0 bottom-0 p-3"> ← sibling of the wash
      <h3 class="line-clamp-2 text-sm/tight font-semibold text-white">…</h3>
      {subtitle && <p class="text-xs text-white/80">…</p>}
    </div>
  </Link>
  {playLink && <Link … rounded-full bg-primary Play …>}
</article>
```

The contract:

- Titles clamp at 2 lines, with an optional muted second line under them — a
  year, or an episode's `S1 E4 · Name`. The scrim they sit on starts near the
  middle of the card and is near-opaque through its lower half, so a two-line
  title never lands on the poster's own lettering; `PosterCardSkeleton` shares
  the same class so the two cannot drift.
- **The wash and the play control travel together.** In `PosterCard` that is
  one `playLink` prop: a card with nothing single to play (`ShowCard`,
  `InTheatersCard`, `MusicianCard`) omits it and renders neither, rather than
  washing out on hover for nothing.
- The play control is an icon alone, so its `playLabel` is the only wording
  it carries, and it follows the app-wide rule: "Play <title>", or
  "Resume <title>" when a position is saved. `MovieCard` derives the verb from
  its `watchProgress`; `ContinueWatchingEpisodeCard` always resumes. Every card
  in the home Continue Watching row therefore reads "Resume …", whatever its
  kind.
- Every hover reveal also fires on `group-focus-within` (§1.7).
- Progress is always `WatchProgressBar`, and the percent comes from
  `watchProgressPercent` in `lib/format.ts` — the one definition shared by the
  bar and the `"N% watched"` in the card's link label, so the bar itself stays
  decorative.
- **Prefetch on hover**: the `onMouseEnter`/`onFocus` wiring lives in
  `PosterCard` behind an `onPrefetch` prop; each card supplies the query
  (`MovieCard` the movie details, `ShowCard` and `ContinueWatchingEpisodeCard`
  the show details). `InTheatersCard` passes none — an unreleased title has no
  detail query to warm.
- Rating chips tier via `criticRatingClass` / `audienceRatingClass` in
  `lib/rating.ts` (`bg-aurora` ≥7 / `bg-aurora/80` ≥5 / `bg-muted`),
  rendered with the `Badge` primitive where no list semantics are needed
  (§1.6). The
  TMDB community score is always the labelled `TmdbScoreBadge`, never a tiered
  chip: it is a different metric.

#### Grids and rails

The canonical library poster grid is `LIBRARY_POSTER_GRID_CLASS`
(`grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`). Home
sections instead use the shared auto-fill grids (`HOME_POSTER_GRID_CLASS` /
`HOME_ALBUM_GRID_CLASS`, both
`grid-cols-[repeat(auto-fill,minmax(min(7.5rem,100%),1fr))]`) —
**auto-fill, not auto-fit**, so a sparse section keeps its cards near the
track's minimum width instead of stretching one poster across the whole content
column; pinned by `constants-contracts.test.ts`.

True horizontal rails (cast, chapters, extras, the seasons tab strip) are
the shared `ScrollRail` (`components/shared/ScrollRail.tsx`): a wrapper on
`SCROLL_RAIL_BLEED_CLASS` that bleeds to the viewport edge at each
breakpoint, around a scroller on `SCROLL_RAIL_SCROLLER_CLASS` (thin glacier
scrollbar, the padding restored). It says when there is more: a
`from-background` fade on each edge that still overflows, and outline
prev/next arrows ("Scroll cast left") that appear on hover and on
focus-within (§1.7) and page by 80 % of the visible width, smoothly unless
motion is reduced. Both arrows stay rendered while either side overflows,
the exhausted one `aria-disabled` and inert rather than removed, so a
keyboard user who pages to the end keeps focus. A touch-first device gets the
fades and no arrows — swiping is the affordance there. `asChild` makes a
`<ul>` the scroller so it keeps its list role, label and (for the cast rail)
its `tabIndex` and ring; the seasons strip wraps the Radix `TabsList` as a
direct child so its tab/panel wiring is untouched. The cast and extras rails
are the shared `CastSection` and `ExtraVideosSection`, used by both the
movie and show detail pages.

#### Library pages

The Movies, TV Shows and Music pages are the same page with different nouns,
so the page-level pieces are shared components in `components/shared/`, each a
`Library*`: `LibraryStats` (the labelled count region beside the header; it
takes `figures`, one or several, and joins them into a single
`Library statistics: 42 movies` / `…: 3 albums, 40 tracks, 2 musicians`
name; the row it shares with the More menu does not wrap — the region is
`min-w-0 flex-1`, so at phone widths its figures wrap onto a second line
while "More options" stays at the right end of the first), `LibraryMoreMenu` with `RefreshLibraryMenuItem` and
`RequestMediaMenuItem` (the "More options" dropdown; every library has at
least Refresh Library, which refetches the page's cached queries and toasts —
Movies adds Request Movie and Music adds Request Album / Request Track beside
it, each a `RequestMediaMenuItem` that stays disabled until its provider's
status answers and states the reason in its accessible name, its tooltip and
an `sr-only` tail, since a disabled menu item otherwise announces only its
label; the page owns the status query, because one mounted inside the dropdown
would not fetch until the menu opened), `LibraryAllTab` (the paginated grid and its skeleton),
`LibraryGenresTab` (the genre-chip facet with focus restoration after Clear,
and its skeleton), `LibrarySortToggle` (the single A–Z / Z–A button) and
`LibraryEmptyState` (the minimal empty variant, §3.4). A page supplies what
differs: the card renderer, the lowercase nouns for copy and announcements, its
prepared `queryOptions()`, the tab triggers, and navigation callbacks — the
page keeps the typed `navigate({ to, search })`, so the shared tabs never
learn a route. `LibraryAllTab`'s `sort`/`onSortToggle` pair is optional: an
API that sorts (movies, shows) passes both, one that does not (albums,
musicians) passes neither and gets no toggle. Its `gridClassName` and
`skeletonCard` swap the 2:3 poster grid for another card geometry — the Albums
tab keeps the poster grid with `AlbumCardSkeleton`, the Musicians tab passes
its five-column round-thumb grid with `MusicianCardSkeleton`. The tab owns one
toolbar — page info on the right, then the sort toggle, with `toolbarStartSlot`
for anything a page puts on the left — and renders it identically while
loading and loaded, reserving its height while loading so the grid does not
shift down when the data lands (§3.4). A tab with neither a sort toggle nor a
start slot drops the row once it knows it has a single page, nothing, or an
error — a failed refetch included, whose stale page count no longer applies —
rather than holding an empty band above the grid; that one settle is the
only time such a tab's grid moves. The liked-movies view is a
`LibraryAllTab` whose `toolbarStartSlot` carries its "Back to playlists" link
and count, so it inherits the tab's out-of-range page clamp and keeps that
link reachable while loading, empty and errored. The movie playlist page is
the same shape: its header above a `LibraryAllTab` whose `toolbarStartSlot`
names the list and whose `emptyMessage` scopes the empty copy to the playlist
rather than the library. The Playlists tabs themselves stay local to their
pages, as the Tracks tab does to the music page; a new library page composes
the same parts. The
search page's category tabs reuse `LIBRARY_POSTER_GRID_CLASS`,
`LoadErrorAlert` and `PosterCardSkeleton` but stay page-local: their result
count line, "No albums match 'q'" copy and non-grid tracks list are not a
library tab.

#### Detail pages

Movie, show and in-theaters detail pages are built from shared parts —
`DetailHero` (+ `DetailTitleHeading`, `DetailGenresList`, `DetailBackdrop`),
`DetailSkipLinks`, `DetailSkeleton`, `OverviewSection`,
`AboutSection`/`AboutRow`, `CrewDisclosure`, `TmdbScoreBadge` — with each
media type supplying only its own metadata chips, key-crew summary, and about
rows.
Adding a media type means supplying those three, not building a fourth page.

`DetailTitleHeading` keeps a text-node space between the title and the
parenthesised year, since the two are flex items and would otherwise be read
as one word. `CrewDisclosure` expands the full crew in place as a one-, two-
or three-column grid — the window is the page's only scroller (§3.1), so no
section opens a nested scroll area.

The movie hero's `actionsSlot` (`MovieDetailsHeroActions`) is a 2×2 grid
below `sm` — Play, Watch, Like, and the More menu with a visible "More" label
beside its icon — and a wrapping row from `sm` up. An action row that relies
on `flex-wrap` with content that cannot shrink drops its last control onto
a lonely row at 390 px; a grid never does.

The album and musician pages keep their own hero anatomy — a square cover or
a round thumb beside the title, over a decorative 21:9 band, rather than
`DetailHero`'s poster — and share the music-specific parts in
`components/music/`: `MusicDetailBackdrop` (the aria-hidden band, on
`usePosterFallback`), `MusicDetailSkeleton`
(`variant="album" | "musician" | "playlist"`: the album and musician variants
are one hero geometry with the art shape and hero rows switched; the playlist
variant mirrors the playlist page instead, which has no backdrop band and no
overlap, just a square cover beside the title over the rows), and
`MusicDetailBackNav` (the `nav` "Page navigation" landmark back to the owning
`/music` tab, also used by the playlist page). All three pages, the playlist
included, guard through `MediaDetailGuard` (§3.4), so a malformed id, a failed
load and a missing payload each render `MediaNotFound` with a link back to the
owning tab, never a bare heading or a lone spinner.

#### Seasons and episode rows

A long control strip scrolls on the rail bleed rather than wrapping.
`ShowSeasonsSection` is a full shadcn `Tabs` pair — a `TabsList` of fully
named triggers ("Season 3", "Specials") on `LIBRARY_TAB_TRIGGER_CLASS` inside an
`overflow-x-auto` rail, and one `TabsContent` panel holding
`ShowSeasonEpisodeList` — so every tab's `aria-controls` resolves to a real
`tabpanel`.

The list renders the selected season as divided rows in the album track-list
idiom inside `DETAIL_TRACK_LIST_CONTAINER_CLASS`. It owns its own skeleton,
empty and error states because its query is separate from its page's, and
announces the loaded and empty ones through `LiveAnnouncer` since the tab
change itself says nothing; the error state is left to its alert (§3.4).

Each episode row is playable: a round accent `Play` icon link
(`buttonVariants({ variant: "accent", size: "icon" })`, min 40px) named in full
— "Play S1 E3 The Thaw", or "Resume …" when a position is saved — leading
to `/tv-shows/$id/episodes/$episodeId/play`, and a ghost `Check` toggle
(`aria-pressed`, "Mark S1 E3 as watched/unwatched", `text-success` when on)
that flips watched state optimistically on the season query
(`EpisodeWatchedToggle`). Resume state is the shared `WatchProgressBar` strip
over the still plus a "1 hr 35 min left" note in the metadata line, so the
position is never conveyed by colour alone; a watched episode swaps the strip
for an outline `Badge` reading "Watched". Remaining time reaches seconds only
inside the last minute ("45 sec left"), and — like every other abbreviated
readout (§1.7) — the note is `aria-hidden` beside an `sr-only` span speaking it
in full ("1 hour 35 minutes left"). The movie hero's resume strip is the same
pair.

The show hero's `actionsSlot` is `ShowDetailsHeroActions`: one accent button
that reads the same season query and picks its target for the viewer — "Resume
S1 E3" for the first partly watched episode, else "Play S1 E4" for the first
unwatched, else the season's first episode. It renders nothing while the season
is unknown or empty.

### 3.3 Images

- **Movie/TMDB images are same-origin proxied**: build URLs with
  `buildTmdbImageUrl(path, size)` (`lib/tmdb-image-url.ts`) →
  `/api/tmdb/images/{size}{path}`. Sizes are the `TMDB_*_SIZE` constants in
  `lib/constants.ts` — `w500` posters and episode stills, `w1280` backdrops,
  `w185` profiles, `w92` network logos; the proxy accepts only those plus
  `original`. Music images go through `getMediaImageUrl()`
  (`lib/media-image-url.ts`). Never hit `image.tmdb.org` directly from the
  client.
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
  screen — `defaultPendingComponent: AppLoadingScreen` in `src/App.tsx`
  (`role="status"`). Within a page, each query renders a **skeleton that
  matches the real layout's grid geometry exactly** (same columns, same aspect
  boxes) so content arrival causes no layout shift — see the shared
  `DetailSkeleton` (`withActions` mirrors whether the real hero has an actions
  row; pages append their own below-the-fold geometry as children),
  `MusicDetailSkeleton` (§3.2), and the `LibraryAllTabSkeleton` /
  `LibraryGenresTabSkeleton` that live in the same files as the grids they
  mirror. A skeleton mirrors the **grid only**; chrome whose presence depends
  on the response — a library tab's page info — belongs to the component that
  renders it in every state, never to the skeleton, which cannot know how many
  pages there are before the query resolves. A grid skeleton repeats one card placeholder that lives beside the
  card it mirrors — `PosterCardSkeleton` in `PosterCard.tsx`,
  `AlbumCardSkeleton` and `MusicianCardSkeleton` in their card files — so a
  card and its placeholder change together. Skeleton geometry is authored
  directly in each loading layout with muted boxes and the shared
  `MOTION_LOADING_STATE_CLASS`, always beside the layout it must mirror — a
  skeleton moves with its layout, never on its own. `ui/spinner.tsx` (`role="status"`) uses
  `MOTION_SPINNER_STATE_CLASS`. Skeleton layouts hide their visuals with
  `aria-hidden` under a single `role="status"` + `sr-only` label.
- **A section whose query the loader awaits renders nothing instead of a
  skeleton.** `ContinueWatching` is the case: the home loader has already
  resolved the query by the time the section mounts, so a skeleton would only
  add a block that collapses a frame later. It returns `null` while pending
  and when empty. This applies only to loader-awaited sections — a query that
  can genuinely still be in flight gets a skeleton.
- **Empty, two variants, chosen by what is empty.** Minimal:
  `LibraryEmptyState` — centered `text-muted-foreground` with a large faded
  lucide icon and one sentence — for a *tab or filter* that has nothing in it
  (search no-results, a genre, the Liked and Playlists tabs, the notifications
  panel). Rich: the shared `EmptyState.tsx` — gradient icon orb
  (`size-20 rounded-full bg-linear-to-br from-muted via-muted to-primary/30`),
  title, description, optional pill CTA, optional `bordered` wrapper — for a
  *whole library* that is empty (the home sections, a show with no episodes).
  Adjacent tabs on one page therefore share one style, and an action gets one
  call to action: a tab whose toolbar already offers "New playlist" does not
  repeat it inside its empty state. Empty states announce via `LiveAnnouncer`
  (`LibraryAllTab` says `No {plural} found` beside its `LibraryEmptyState`).
- **Error, by shape.** Detection is uniform:
  `isError || isApiFailure(data)` (`lib/is-api-failure.ts`, reading the API
  envelope `{ error, message, data }`). Which component renders it depends on
  what failed:
  - A query inside a page → `LoadErrorAlert` (`role="alert"`,
    `border-destructive/25 bg-destructive/10 text-destructive`, "Try again" →
    `refetch()`).
  - A home section's query → `SectionErrorAlert`, the same destructive tint as
    a shadcn `Alert`, rendered for you by `HomeMediaSection`.
  - A Settings card → `SettingsErrorCard` (and `SettingsLoadingCard` for its
    pending state), so the card keeps its place in the page.
  - The four ways a detail page fails to show its subject — an invalid id, a
    failed request, one still in flight, an empty response — are
    `MediaDetailGuard`, which every `$id` route wraps its content in. Three of
    the four are dead ends, so each renders `MediaNotFound` (title "Not found"
    for a 404 or an empty response, "No access" for a 403, "Error"
    otherwise); its destination is
    a named key (`music`, `moviePlaylists`, …) carrying both the route and the
    words on the link, so the two can never disagree and a destination may
    carry search params.
  - A missing resource → `MediaNotFound`: a destructive `Alert` plus a
    **required** "Back to Movies/TV Shows/Music/Home" outline link, so the
    page never dead-ends.
  - A mutation → **toast** via `toast-helpers.ts`, never inline.
  - **The UI owns the words for failures it can name.** `apiRequest` stamps the
    HTTP `status` on every failure envelope it returns, and a surface maps the
    statuses it understands to a sentence — a 404 on a detail page reads "We
    couldn't find that movie.", a 403 "You don't have access to this
    playlist.", a 400/401 on login "The email or password is incorrect.", a
    403/404 on a watch room "This room no longer exists or you were not
    invited." A page about one subject (`MediaDetailGuard`, the trailer
    dialog) then uses a fixed sentence of its own for everything else
    ("Something went wrong while loading this movie. Please try again later.")
    and shows no server text at all. List and section loads go through
    `apiErrorMessage`, which keeps the server's message only for a 4xx other
    than 404 and otherwise uses the surface's own sentence, so a 404 (whose
    body `apiRequest` replaces with a canned string) or a 5xx (internal error
    text) never reaches the screen. Never show the client's canned "404 - The
    resource…" or "500 - A network error…" strings or a lowercase server
    constant as the copy.

  Because the erroring subtree often unmounts its own live region, error
  surfaces carry `role="alert"` and announce themselves — never repeat one
  through `LiveAnnouncer` as well. Placeholder sections (`ComingSoon`) reuse
  the EmptyState language.

### 3.5 Playback surfaces

Playback code is high-risk (see `docs/ffmpeg.md`) — style changes here still
require the full playback test pass.

- **Video player** (`components/playback/VideoPlaybackPage.tsx`, mounted by
  `routes/_auth/movies/$id/play.tsx` and
  `routes/_auth/tv-shows/$id/episodes/$episodeId/play.tsx`, + `VideoPlayer.tsx`
  + `PlayerControls.tsx`): one page for movies and TV episodes, addressed by a
  `PlaybackMediaRef` (`{ kind, id }`); the route supplies the header (film or
  TV icon, title — "Show · S1 E3 · Episode" for TV — artwork, not-found copy,
  where Back falls back to, and the `posterUrl` — the movie backdrop or the
  episode still — the `<video poster>` shows in the frame until the first
  frame paints, so the page never opens on a black box). It renders
  **in-shell** as a windowed player
  (header bar: media icon + title + Back; controls footer below the video —
  progress group, time readouts in `tabular-nums`, rewind / play-pause /
  fast-forward cluster, quality chip, chapter menu, volume, fullscreen).
  **One time readout per width**: from `sm` up it sits in the transport row
  and the progress bar's own labels are hidden; below `sm` the row cannot hold
  it beside seven controls, so the bar shows the labels and the row hides its
  pair and the decorative quality chip (Playback Settings still names the
  mode). Both readouts take `displayedDuration` — the catalog length while
  an HLS element has not yet reported one — so the two never disagree.
  In immersive/fullscreen mode (`isImmersiveViewport` /
  `chromeFullscreenMode`) the container goes `fixed inset-0 z-50`, chrome
  becomes absolute overlay panels (`MOTION_PLAYER_CHROME_PANEL_CLASS`,
  `bg-background/95 backdrop-blur-lg`) and **auto-hides on idle**
  (`useIdleControls`), sliding back on pointer/touch/key input. A `sr-only`
  paragraph documents the keyboard map (Space/K, J/L, arrows, M, F, Esc),
  and the controls name their shortcuts — both dropped on touch (§1.7);
  `ResumeDialog` offers resume vs. start-over (which clears the saved
  progress). A page opened mid-media (a restored tab, a reload, Back into the
  player, a chapter link) offers it only when the saved position is more than
  `RESUME_AHEAD_THRESHOLD_SEC` past the URL's start, and then the alternative
  is "Play from {start}", which keeps the progress; an up-next hand-off is
  never interrupted. No progress is saved while the offer is undecided, so
  leaving or switching tabs cannot overwrite what it offers. Announcements
  via five
  `LiveAnnouncer`s (play/pause state, capacity waiting, chapter jumps,
  direct-play fallback, and HLS session recovery — the watch room announces
  recovery the same way). Fatal playback errors self-announce: the status
  screen's non-loading variants and the watch-room error box carry
  `role="alert"` because the player subtree (and its live regions) unmounts
  before they appear.

  **The up-next card** (`UpNextOverlay.tsx`) appears when a TV episode ends and
  the header names a `next_episode`; movies never show it. It is a `section`
  labelled "Up next", anchored to the bottom of the video, holding the episode
  still, "S1 E4 · Name", a `tabular-nums` "Playing in Ns" / "Resuming in Ns"
  countdown (`UP_NEXT_COUNTDOWN_SEC`) and two buttons: primary "Play now" /
  "Resume now" (focused on appear) and outline "Cancel". It announces itself
  once, politely, never per tick.

  While it stands, the card **owns the player's keyboard**, exactly as
  `ResumeDialog` does, so Enter and Space activate the focused button instead
  of toggling playback. In fullscreen the transport chrome yields the bottom
  edge to it — both are absolutely positioned there and the chrome would paint
  on top — and the card stops pointer events reaching the click-to-toggle
  surface, which would otherwise restart the episode that just ended.
  Cancelling, or seeking back into the episode, retracts the card, restores the
  chrome, and returns focus to the player region.

  The hand-off pushes to the next episode's play route with `start` at its
  saved position and `autoplay=true`, which the player treats like a rebase
  resume: it plays on the first `canplay`, then drops the spent flag from the
  URL (replace) so a reload does not replay it. If the browser refuses
  autoplay, the viewer sees the paused player and presses Play.
- **Trailer pre-roll** (`components/playback/PrerollPlayer.tsx`, YouTube
  behind `useYouTubePlayer` with its own controls off): an opt-in phase
  *inside* the movie player, never a chain of `/trailer` navigations, so
  fullscreen, the keyboard and one Back exiting the whole play session stay
  coherent. It runs only for a movie starting from the beginning: the resume
  decision resolves first, Resume bypasses it, Start over and a fresh play run
  it; TV episodes, watch rooms, chapter links and Continue Watching never see
  it. That is decided once, from how the page opened, so seeking back to 0:00
  in a movie opened mid-way never brings it up. The queue comes from `GET /api/movies/{id}/preroll` (the server owns
  selection), fetched once per play with a short timeout and never refetched
  on focus or reconnect; an empty or failed answer starts the movie as before,
  paused, while a pre-roll that played starts the movie on its own (a
  direct-play fallback during the warm-up waits for it). The layer is a
  `section` "Trailers before the movie" over the movie surface, which stays
  **unmounted** while trailers play (mounting it starts the HLS session) and
  is warmed up only in the last trailer's final `PREROLL_MOVIE_WARMUP_SEC`
  (once YouTube reports the trailer's duration; without one, no warm-up);
  it stops clicks so the fullscreen click-to-toggle never reaches the movie,
  and a transparent layer over the iframe keeps focus in our document. Its
  chrome replaces `PlayerControls` in the footer slot with the same panel
  classes (so it hides on idle in fullscreen): "Trailer X of N · title" in
  `tabular-nums`, `ProgressBar variant="trailer"`, the primary play/pause,
  outline **Skip trailer** (focused when the pre-roll appears), accent
  **Start movie**, and fullscreen (the expanded view where the browser lacks
  element fullscreen, never the warmed movie's video-only fullscreen). Keys
  follow the player's conventions and go through `useShortcutHints`:
  Space/K, **N** skip, **S** start movie, F, Escape. Each trailer is announced once ("Trailer 1 of 2: Title") through
  `LiveAnnouncer`; an embed error (removed, private, embed-disabled) skips the
  trailer silently, an unavailable player ends the pre-roll, and no watch
  progress is saved until the movie starts. No feature-presentation card and
  no link to the trailer's movie.
- **Trailer player** (`routes/_auth/trailer.tsx`, YouTube behind
  `useYouTubePlayer`): one `DialogFullscreenContent` whose view swaps in place
  — loading, load error, no trailer, the player, a playback error. Every view
  wears the same `TrailerHeader` (film icon, title, "Close trailer"), so the
  way out never moves between states; only the player view's header holds the
  `DialogTitle`, the others name the dialog from their own heading. Each view
  focuses its primary control ("Close trailer (Escape)" — "Close trailer" on
  touch, §1.7 — or "Try Again" on an error) when it first appears, not only when the dialog opens: Radix's open
  auto-focus runs once, and a swap that unmounts the focused control would
  otherwise strand focus on the dialog container.
- **Audio player** (`AudioPlayer.tsx`, app-wide via `AudioPlayerContext`;
  chrome split into `NowPlayingDialog`, `MiniPlayerBar`, and the shared
  `PlayerTransportControls`, with `useAudioPlaybackKeyboard` and
  `useAudioMediaSession` behind it):
  starting a **new** track opens the **fullscreen Now Playing** view
  (`DialogFullscreenContent`, gradient `from-background via-muted
  to-background`, large art with disc-icon fallback, transport + volume +
  "Track N of M"); "Minimize player (Escape)" (no suffix on touch, §1.7)
  collapses it to the **docked
  mini bar** (`fixed inset-x-0 bottom-0 z-40 bg-background/95 backdrop-blur`)
  with track info, transport, close, and a bottom progress strip; below `sm`
  the like and close buttons give way so the title keeps room to read (both
  stay reachable from the fullscreen view). Every track row carries the
  `TrackActionsMenu` (Add to Playlist, then the links that lead somewhere
  else: an album row offers "Go to Artist", a musician row "Go to Album"),
  so a track can join a playlist from wherever it is listed. The bar
  persists across navigation; the current track's row in lists is
  highlighted (`text-primary` title + tinted row + pause state), and
  clicking that row **toggles play/pause in place** — it never rebuilds the
  queue or re-opens the fullscreen view (the header-level "Play all" /
  "Shuffle" buttons are the explicit start-over entry points, and they
  rewind to 0:00 even when their first track is the one already playing).
  Sliders are native ranges (§1.7). Global keyboard map (mirrors the video
  player's aliases): Space/K toggle, J/L and ←/→ seek ±10s, ↑/↓ volume
  (unmuting as it goes), N/P (and the `MediaTrackNext`/`MediaTrackPrevious`
  media keys) next/previous, R/Home/0 restart, M mute. Inside the player's
  own chrome these keys always control playback — Space pauses even when a
  player button holds focus (Enter still activates it); outside the player, Space
  and navigation keys are left to the focused control (buttons, tabs,
  radios), and the player's native sliders keep their own arrow/Home
  handling.

### 3.6 Toasts & notifications

- **Sonner wrapper** (`ui/sonner.tsx`): theme-synced via
  `useSyncExternalStore(subscribeTheme, getActiveTheme)`; `richColors`,
  `closeButton`, top-right; toast surfaces tokenized with `!` overrides
  (`!bg-card`/`!bg-muted`, `!border-success/50`, `!border-destructive/50`).
  Fire success/failure through `lib/toast-helpers.ts`
  (`showActionFailed(...)` etc.), not ad-hoc `toast()` calls.
- **NotificationBell** (header): ghost icon button with a glacier unread
  badge pill ("99+" cap; count also in the `aria-label`), opening a `w-80
  bg-card` popover — header row with "Mark all read", `max-h-96` scroll body
  with spinner / a compact `LibraryEmptyState` ("You're all caught up." under
  a faded `BellOff`) / `divide-y` list states. Unread
  rows tint `bg-muted/40` with a glacier dot; rows show type label, message,
  relative time, per-row dismiss. Unread count polls every 30s; the list
  query is `enabled` only while open.

### 3.7 Forms & inputs

- Settings pages are shadcn Cards on `SETTINGS_CARD_SURFACE_CLASS`
  (`border-border/50 bg-muted/30`) with a `SettingsCardHeader` (glacier lucide
  icon + title + description), labeled inputs on `SETTINGS_INPUT_CLASS`,
  helper text below (`text-muted-foreground text-xs`), and per-field or
  per-card Save buttons; destructive areas get the tinted "Danger Zone" card
  (`border-destructive/*` + destructive text + outline destructive actions).
  A card's own loading and error states are `SettingsLoadingCard` and
  `SettingsErrorCard` (§3.4), so the card keeps its place in the page.
- **The Save bar (`SettingsSaveBar`) is opaque, always there, and sticky only
  while dirty.** It renders at the end of every server-saved form on a solid
  `bg-card` (floating surfaces are opaque, like the mini player and dialogs —
  a translucent bar let the form read through it), so a keyboard user can
  always find it; it lifts into a sticky bar at `stickyClassName` (`bottom-4`,
  or `MINI_PLAYER_CLEARANCE_BOTTOM_CLASS` above the mini player) only while
  the form differs from what is saved, and its Reset and Save are disabled
  when there is nothing to do, with the status line reading "No unsaved
  changes" — unless a success or error message is standing ("Library paths
  saved."), which wins. Below `sm` it is one row: the status, an icon-only
  Reset keeping its accessible name, and Save. The Playback page's one bar
  belongs to the admin "Server" card and renders `embedded` in that card's
  `CardFooter`, so it cannot be mistaken for saving the device and account
  cards above it, which save as they change.
- **The Settings tab strip** is two columns on a phone (the fifth tab
  spanning both), five equal tabs filling the content width from `@md` (on
  the tighter `px-1.5` until the card fits to content), and the library pages'
  fit-to-content card only from `@2xl`; a narrow 2+2+1 card beside empty
  space at tablet width read as orphaned.
- **Card section titles are real headings**: render `CardTitle` with `asChild`
  wrapping an `<h2>` (login's is the page `<h1>`) so card-sectioned pages are
  navigable by heading.
- The header search + login inputs use the frosted light treatment from
  `lib/input-styles.ts` — the sanctioned raw-color exception (§1.2); its
  contract is pinned by `shared/input-styles.test.ts`.
- File inputs, checkboxes, selects: use the vendored primitives; selects in
  dialogs rely on the `data-slot` contract (§1.6). Destructive confirmations
  go through the shared `ConfirmDialog`, and paged lists through
  `LibraryPagination` — neither is re-implemented per page.
- Form-level failures toast; field-level validation renders inline with
  `aria-invalid` (styled by the Button/Input base classes).
- **Device-scoped settings apply instantly and carry no Save bar.** Settings
  that belong to the browser rather than the account (theme; the playback
  preferences in `lib/playback-preferences.ts`) persist to localStorage on
  change. Because the Save bar that used to confirm the write is gone, such a
  card must: state its scope in the `SettingsCardHeader` description ("Saved
  in this browser only…"), and confirm each change through `LiveAnnouncer`
  so the write is not silent for screen-reader users. The confirmation must
  reflect what actually happened — localStorage can refuse the write (private
  browsing, quota), so the writer returns whether it persisted and the card
  says "applied for this session only" instead of claiming a save. Because the
  announcement is one-shot while the refusal is a standing browser condition,
  such a card also renders a visible `text-destructive` notice for as long as
  it lasts. Do not mix the models inside one card — split by ownership,
  as Settings → Playback does (two "this device" cards, one account card,
  one admin-only "Server" card with the only Save bar, hosted in that card's
  footer).
- **Account-scoped settings also apply instantly, through the API.** A
  setting that belongs to the account rather than to one browser or to the
  server ("Trailers before movies", `TrailerPreferencesCard`) saves every
  change at once with an optimistic mutation: the control reflects the
  change immediately, `LiveAnnouncer` confirms it once the save lands
  ("…turned on. Saved to your account."), and a failure toasts through
  `showActionFailed` and rolls the control back. No Save bar. The
  `SettingsCardHeader` description states the scope ("Saved to your account,
  so it follows you across browsers and devices"), as the device cards state
  theirs. Its on/off control is the shared `SwitchField` (a real
  `role="switch"` button with its description tied in by `aria-describedby`),
  the same row the General settings card uses.
- **A setting the server may not honor says what is in force.** Under the
  Server card's hardware acceleration select, a notice read from the saved
  settings (not the form) names the device transcodes actually run on:
  muted helper text when it is the stored device, and `text-destructive`
  with the probe's reason for as long as the startup probe refuses the stored
  one. It also names the transcode cap whenever `max_transcode_height` is
  below the tallest catalog profile.
