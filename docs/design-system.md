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
| Fields share their looks; a dialog's surface, width, elevation and title tone are props; neither takes a colour or height in `className` (§1.6) | `web/src/test/ui/fields.test.tsx`, `web/src/test/ui/dialog.test.tsx`, ESLint `no-restricted-syntax` (**error**) |
| App-wide layers and motion durations are written as tokens, not numbers (§1.4, §1.5) | ESLint `no-restricted-syntax` (**error**) |
| Shared class-string constants keep their contracts | `web/src/test/lib/constants-contracts.test.ts` |
| Primitives and the baseline pages render the same pixels as the committed baselines, pixel for pixel, both themes (§2.5) | `web/e2e/visual/*.visual.ts` (CI job `test-visual`) |
| The component gallery renders, and stays out of the production bundle (§2.5) | `web/src/test/gallery/gallery.test.tsx`, CI marker grep, `web/e2e/boot.spec.ts` |
| No raw Tailwind palette classes (all 22 color families) | ESLint `no-restricted-syntax` (**error**) in `web/eslint.config.js` |
| Class order, duplicates, unknown and non-canonical classes — in `className`, inside `cn()`, and in every class constant and variant record (§2.3) | `eslint-plugin-better-tailwindcss` (**warn**; `bun run lint` runs `--max-warnings 0`, so warnings still fail CI) |

---

## 1. Foundations

### 1.1 Identity & approach

Igloo is an "icy glacier" themed media center. The palette is a cool glacier
blue primary with a sparing warm amber ("aurora") accent, on a deep navy canvas
(dark, the default) or a cool near-white canvas (light).

The system is implemented with:

- **Tailwind CSS v4** — configuration lives in CSS (`@theme inline` in
  `web/src/assets/styles.css`); there is no `tailwind.config.js`.
- **Igloo's own primitives** in `web/src/components/ui`, built on the
  consolidated `radix-ui` package and styled with `cn()` (`clsx` +
  `tailwind-merge`, from `web/src/lib/utils.ts`). They began as shadcn/ui
  "new-york" components and are maintained here (§2.2).
- **Semantic CSS custom properties** in OKLCH, defined once per theme
  (`:root` = light, `.dark` = dark) and consumed through utilities like
  `bg-card`, `text-muted-foreground`, `ring-ring`. The token rules are
  **generated** from `src/lib/theme-tokens.ts` (§2.4).
- **Igloo's own enter/exit animation utilities**
  (`src/assets/animations.css`), with a strict `motion-reduce:` discipline
  (§1.5).
- **lucide-react** for all icons, with two exceptions. The first is the
  **brand mark**: `components/app/BrandMark.tsx` draws the igloo from
  `lib/brand-mark.ts`, the same glyph the app icons are generated from
  (§2.4), in `currentColor` with transparent cut-outs so the parent picks the
  tile: the sidebar shows it on a `bg-primary` tile, the login card and
  loading screen in a `bg-muted` orb tinted `text-primary`. Never stand in a
  lucide glyph or a letter for the logo — one mark everywhere, as the
  streaming apps do. The second is **third-party service marks** (Spotify,
  Apple Music): their paths come verbatim from Simple Icons (CC0) at a
  pinned version noted beside each, they are `aria-hidden`, and they only
  ever sit beside the service's visible name.

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
| `border` | `#24354E` | `#CBD9E8` | Borders |
| `input` | white @ 8% | `#CBD9E8` | Input borders |
| `ring` | `#38BDF8` | `#0EA5E9` | **The one focus color** |
| `destructive` / `-foreground` | `#F87171` / `#08131F` | `#B91C1C` / `#FFFFFF` | Danger / delete |
| `aurora` / `-foreground` | `#F59E0B` / `#08131F` | same | Warm accent (sparing) |
| `success` / `-foreground` | `#34D399` / `#08131F` | `#167050` / `#FFFFFF` | Success state |
| `accent-teal` / `-foreground` | `#2DD4BF` / `#08131F` | `#0D9488` / `#08131F` | Secondary accent |
| `media-fallback` / `-foreground` | `#0F1A2E` / `#8094AE` | same | Tile under over-media chrome when the media is missing |
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
  with the theme); keep them literal, don't hunt for tokens. When the media
  itself is missing, the box they sit on is `MEDIA_FALLBACK_SURFACE_CLASS`
  (`bg-media-fallback` + `text-media-fallback-foreground`): a navy tile that
  is the same in both themes, like the app icon's canvas, so the scrim and
  the white title read as they do on a poster — a `bg-muted` box is
  near-white in light and turned them into a grey-to-black smear.
  `PosterCard`, `DetailBackdrop` and the hero's poster figure use it. A hero
  whose title has a poster but no backdrop layers that poster over the tile,
  blurred (`blur-2xl`) and scaled past the edges (`scale-110` inside an
  `overflow-hidden` band), so the band still reads as media and the scrims
  stay literal; the tile shows through if the poster fails to load. A
  placeholder with foreground text beside or below it (album covers,
  musician thumbs, episode stills, the watch-room thumb) keeps `bg-muted`.
  The one exception
  is the **login backdrop**, which swaps between a bright (`login-bg-light.webp`)
  and dark (`login-bg-dark.webp`) photo by theme, so its frost/darkening scrim
  gradient and frosted-card border in `login.tsx` are deliberately
  `dark:`-variant, literal white/slate in light and `background`-token in dark.
- One deliberate brand exception exists: the `SPOTIFY_BRAND_*` constants in
  `lib/constants.ts` (Spotify green, documented there, carried past the lint
  ban with inline `eslint-disable` comments). The Apple Music mark takes no
  brand colour: it is drawn in `text-foreground`, like Apple's monochrome
  badges, so no second brand colour enters the palette.
- **A border is a quiet line.** It separates two surfaces and never outranks
  what they hold: against the canvas and against a card it sits between
  1.2:1 and 1.55:1 in both themes, and `contrast.test.ts` pins both ends, so
  a token edit can neither erase it nor bring a hard outline back. Dark is
  `#24354E` (1.5:1 on the canvas, 1.27:1 on a card). Light stays `#CBD9E8`
  and is not to be lightened without first giving controls an edge of their
  own: an outline button and a Settings field are white on a white or
  near-white card there, and the border is the only edge they have (a
  disabled Reset button's edge is all but gone at `#DAE4EF`).
- `text-success` is AA-safe on `background` and `card` in both themes
  (light `--success` is a deep glacier green, `#167050`); the pairs are pinned
  by `contrast.test.ts`.

### 1.3 Typography

- **Font stack**: `Inter, ui-sans-serif, system-ui, -apple-system,
  BlinkMacSystemFont, "Segoe UI", sans-serif`, declared once on `body` in
  `web/src/assets/boot.css` (with antialiasing). **Inter is self-hosted**: the
  variable font (`web/public/fonts/InterVariable.woff2`, weights 100–900) is
  loaded by an `@font-face` in `boot.css` with `font-display: swap` — no CDN,
  no npm package, and deliberately no `<link rel="preload">` in `index.html`:
  the 352 KB variable file would compete with the render-blocking CSS, and on
  a Slow 4G profile that cost ~700 ms of first styled paint for ~170 ms of
  earlier Inter (both a few milliseconds on a fast link), while Chromium's
  "preloaded but not used" warning came and went with it. Do not declare
  other font families in components.
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

- **Radius**: `--radius: 0.625rem` (10px). Aliases: `rounded-sm` = radius−4px, `md` = −2px, `lg` = radius, `xl` = +4px.
  Convention: cards `rounded-xl`, buttons/inputs `rounded-md`, chips/pills
  `rounded-full`.
- **Spacing and type stay deliberately untokenized.** Both are the standard
  Tailwind scales as utility literals. Tokenizing them was considered and
  rejected: the values are already consistent in practice, and a token layer
  over a scale that every developer already knows buys indirection, not
  safety. What is tokenized beside color is what the app defines for itself
  and Tailwind has no name for: the radius, the elevation of a floating
  surface, the app-wide layers and the three motion durations (below, §1.5).
  This rule is enforced by review, not a test.
  Rhythm in practice: `gap-4` in poster grids, `gap-6` between card blocks,
  page sections `mt-6 md:mt-8`, shell content padding
  `px-4 py-6 sm:px-6 lg:px-8`.
- **Aspect ratios are part of the design vocabulary**: `aspect-2/3` movie and
  show posters, `aspect-square` album covers, musician thumbs and playlist
  covers, `aspect-21/9` detail-page backdrops (clamped
  `max-h-[min(42vh,22rem)]` at `md+`; the shared `DetailBackdrop` owns the
  scrims and the blurred-poster stand-in for a missing backdrop, §1.2),
  `aspect-video` trailers, extras, episode stills and the up-next
  thumbnail.
- **Elevation**: `shadow-xs`–`shadow-2xl`, plus two shadows of the app's own
  (`SHADOW_SCALE` in `src/lib/theme-tokens.ts`):
  - `shadow-floating` for a surface floating on the canvas — the mini player,
    Now Playing art, the up-next card, media dialogs. Its value differs by
    theme (`--elevation-floating`): 50 % black in dark, 15 % in light, where
    50 % reads as a haze.
  - `shadow-glow` for the glacier glow under a primary control (the brand
    mark, the player's play button, the pill call-to-actions, the active
    tab): the geometry of `shadow-lg` in `primary` at 12 %, a hint of light
    under the control rather than a halo around it. Write the token, never
    a literal `shadow-lg shadow-primary/NN`: the strength is set once, in
    `SHADOW_SCALE`.

  Both are ordinary shadow sizes to Tailwind and to `cn()`: they replace and
  are replaced by `shadow-md` and the like, and the focus ring, which is a
  separate layer, is not disturbed. A shadow-color class beside them is kept
  by `cn()`, but only `shadow-glow` takes it (`shadow-glow shadow-primary/5`
  is the glow at 5 %). `shadow-floating` takes its color from the theme
  through `--elevation-floating`, which Tailwind cannot recolor, so a color
  class changes nothing there; a different strength is a new token. Other
  glows stay literal where their strength is part of the surface:
  interactive media cards on hover (`hover:shadow-xl
  hover:shadow-primary/20`), the EmptyState orb (`shadow-primary/5`). Shadows
  over posters and video stay literal too (§1.2).
- **Layers** (`Z_INDEX_SCALE`): `z-chrome` (40, the header and the mini
  player), `z-modal` (50, dialogs, sheets and the full-window player) and
  `z-popover` (60, menus, selects and popovers, which may open from inside a
  modal). These are the layers that stack across the whole app; stacking
  inside one component (a badge over its poster) stays a plain `z-10`. A
  fixed or portalled surface takes one of the three names, never a number:
  that is what lets a select open over the dialog it sits in without a
  one-off `z-100`.

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
  The three numbers are defined once, as `MOTION_DURATIONS_MS` in
  `src/lib/theme-tokens.ts`: `constants.ts` re-exports them as
  `MOTION_DURATION_*_MS`, and they are the utilities `duration-micro`,
  `duration-standard` and `duration-page`. Class strings use those names,
  never `duration-150` / `-200` / `-300`. Easing stays on Tailwind's
  `ease-out` / `ease-in`.
- **Transitions enumerate properties** — e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]` —
  never `transition-all`.
- **Enter/exit animations** are the app's own utilities, in
  `src/assets/animations.css`: `animate-in` / `animate-out` run one keyframe
  each (`enter`, `exit`) between the resting state and the offsets set by
  `fade-in-0`, `zoom-in-95`, `slide-in-from-<side>-2` and their `-out` twins;
  `fill-mode-*` and `delay-*` complete the set. Rules for that file:
  - It holds only what the app uses (it began as the used part of the
    `tw-animate-css` package, which is removed). A utility that is not there
    (`blur-in`, `spin-in`, `slide-in-from-start`) compiles to nothing, so
    add it to the file with the first use.
  - **`delay-*` is redefined there** to set `animation-delay` as well.
    Tailwind's own only delays transitions, and without it every staggered
    entrance (`delay-75`, `delay-150`) starts at once.
  - Screenshots cannot see any of this, since the visual suite freezes
    animations. `e2e/motion.spec.ts` reads the computed animation name,
    duration, delay, fill mode and enter offsets of a section, a delayed
    section, a select list, a dialog and the sign-in card instead.

  A page entrance both
  fades and lifts (`MOTION_PAGE_ENTER_CLASS`:
  `animate-in fade-in slide-in-from-bottom-2 fill-mode-both duration-page`); a
  section entrance only fades (`MOTION_SECTION_ENTER_CLASS` and its `delay-75`
  staggered twin `MOTION_SECTION_ENTER_DELAYED_CLASS`:
  `animate-in fade-in-0 fill-mode-both duration-standard`) — the split is
  deliberate, since a page full of independently sliding sections reads as
  jitter. Exits use `animate-out` + `fade-out-0`. The two keyframes
  live in `animations.css`; `styles.css` has none of its own.
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
of the system. A call site picks a variant, a size and a shape; it does not
assemble a look from classes.

| Variant | Look | Used for |
|---|---|---|
| `default` | primary fill | the main action of a form or dialog |
| `accent` | primary fill + `shadow-md` | the main action on a page or toolbar |
| `outline` | bordered, canvas fill | the second action beside a main one: Cancel, Reset, Start from beginning, Skip |
| `ghost` | no chrome until hover | an inline action |
| `ghost-muted` | ghost resting in the muted text color | toolbar and row icons: theme toggle, notifications, edit a user |
| `ghost-destructive` | ghost-muted whose hover turns destructive | a row's remove or delete icon |
| `destructive` | destructive fill | the confirming action of a deletion |
| `outline-destructive` | outline in the destructive color, in both themes | a removal that is not its surface's main action: revoke a device |

Sizes are `default`, `sm`, `lg`, `xl`, `icon` and `icon-sm`. `shape="pill"`
rounds any variant fully. The base string carries the focus ring, disabled opacity,
`aria-invalid` styling, a property-scoped 150ms transition with
`motion-reduce:transition-none`, and stamps `data-variant`, `data-size` and
`data-shape`.

- **The quiet second action is `outline`, everywhere.** Cancel in a dialog
  or a confirmation (`AlertDialogCancel`), the save bar's Reset, "Start from
  beginning", the pre-roll's Skip: all plain `outline`, with no classes on
  top. A `secondary` variant with a muted fill and muted text was tried for
  them and removed, because in the light theme it read as a disabled primary
  button (next rule) sitting beside the real one. `Button` is in the lint
  list of primitives styled by props, so a fill, text, border or shadow
  color in its `className` fails the build; a color that is the control's
  state rather than its look (the watched toggle's `text-success`) carries an
  inline exemption that says so.

- **A disabled primary button is grey, not dim glacier.** The primary-fill
  variants (`default`, `accent`) swap to `bg-muted text-muted-foreground` at
  full opacity when `disabled`, with no shadow, so a "Send Request" that
  cannot be sent yet never reads as a second live action beside the real
  one. The other variants keep the base half-opacity. Guarded by
  `test/ui/button.test.tsx`.

- **A call to action is `xl`, 44 px, the touch-target minimum.** The hero
  actions of a detail page (`MovieDetailsHeroActions`, `ShowDetailsHeroActions`,
  the in-theaters Play Trailer), the home billboard's two pills, and the
  buttons of a full-screen status (`PlaybackStatusScreen`, the trailer's no
  trailer and error views) take `size="xl"`, never `lg` plus a `min-h-11`
  override. A status screen's pair is the primary pill with `shadow-glow`
  beside an `outline` pill.
- **A pill stays a pill at every size.** `buttonVariants` merges
  base → variant → size → shape → `className` with `cn` (`twMerge`), so the
  last conflicting class wins. The base sets `rounded-md`, no size sets a
  radius, and the pill shape alone overrides it with `rounded-full`, so call
  sites never re-assert it (guarded by `test/ui/button.test.tsx`). Sibling
  buttons meant to match — the primary and the outline pills of a toolbar
  (`PlayShuffleButtons`, the music index's "Play all" and "Liked tracks") —
  both take `shape="pill"`.
- **Tabs share one look** (`web/src/components/ui/tabs.tsx`): a bordered
  `bg-muted/50` list; the active trigger is a glacier primary-fill pill
  (`data-[state=active]:bg-primary … shadow-glow`). Library pages layer
  responsive grid sizing on top via `LIBRARY_TABS_LIST_CLASS` /
  `LIBRARY_TAB_TRIGGER_CLASS` from `constants.ts`. Tab state lives in URL
  search params (validated in `lib/route-search.ts`), never local state. Each
  index route also strips its defaults on navigation (`search: { middlewares:
  [stripSearchParams(…_INDEX_DEFAULT_SEARCH)] }`, the same objects the schema
  fills in, pinned together by `test/lib/route-search.test.ts`), so a URL
  spells out only what differs from the default: `/movies`,
  `/movies?tab=genres`, `/movies?tab=playlists&playlistsView=liked`.
- **Variants are plain records**: a typed `const` record per axis and one
  function that joins them with `cn(...)`. `badge.tsx` (a `default`/`outline`
  record) is the one-axis model; `button.tsx` (`BUTTON_VARIANTS` ×
  `BUTTON_SIZES`, joined by `buttonVariants()`) the two-axis one. The
  variant's type is `keyof typeof` its record. There is no variant library:
  `class-variance-authority` was removed and its import is a lint error.
- **`buttonVariants()` returns a merged string** — one class per property —
  so an element that is not a `Button` (a link, a Radix trigger) can take it
  as its whole `className` with no `cn()` around it. Wrap it in `cn()` only
  to add classes.
- **A button is a `Button`.** `buttonVariants()` is for an element that is
  not a button but takes its look: a router `Link`, an anchor, or another
  primitive with its own `data-slot` (`PaginationLink`). A raw `<button>`
  that calls it is a lint error: it is a `Button` with the wrapper left off,
  missing the `data-slot`/`data-variant` hooks and anything the primitive
  gains later.
- **Fields share one set of looks** (`ui/field-variants.ts`), chosen with
  `variant` on `Input`, `Textarea`, `NativeSelect` and `SelectTrigger`, so a
  form that mixes them reads as one:

  | Variant | Look | Used for |
  |---|---|---|
  | `filled` (default) | a muted well: `border-border bg-muted` | dialogs, pickers |
  | `inset` | a translucent well on a card: `bg-background/60` | the Settings forms |
  | `glass` | the light glass field of `lib/input-styles.ts` | header search, sign-in, account and users forms |
  | `outline` | the bare bordered field, no fill | inline rename, the quick-connect code |

  `size` (`sm` 32px, `default` 36px, `lg` 40px) sets the height of `Input`
  and `SelectTrigger`; Settings fields are `inset` at `lg`. `SelectTrigger`
  has no `glass`, and its `inset` sits flat (`shadow-none`) where the inset
  `Input` keeps a hairline shadow: both as the Settings forms are drawn
  today.
- **A dialog states what it is with props** (`ui/dialog.tsx`,
  `ui/alert-dialog.tsx`):

  | Prop | Values | Notes |
  |---|---|---|
  | `DialogContent size` | `md` (default), `lg`, `xl`, `2xl` | the `sm:max-w-*` it may grow to; below `sm` every dialog spans the viewport less a margin |
  | `DialogContent elevation` | `raised` (default), `floating` | `floating` (§1.4) for a dialog that opens over media: Resume, Playback Settings |
  | `DialogTitle tone`, `AlertDialogTitle tone` | `default`, `destructive` | `destructive` when the action cannot be undone |

  The surface is the card (`bg-card`) for every dialog; the title takes the
  page's text color and the description is muted, so neither needs a color
  class. `AlertDialogContent` keeps its one width. `DialogFooter` sets the
  gap between its buttons, stacked on a phone and in a row from `sm` up; a
  `gap-*` class on it is a lint error (nine footers once passed
  `gap-2 sm:gap-0`, which left their buttons touching).
- **A primitive's `className` is for layout**: a margin, padding around an
  icon (`pl-10`), a width, scrolling (`max-h-[85vh] overflow-y-auto`),
  `resize-none`, a font (`font-mono`), a text size. A color or height class
  there, or a class constant, is a lint error — the look is the props'. This
  is the rule every primitive family is moving to; the lint list in
  `web/eslint.config.js` (`styledByProps`) names the ones under it:
  `Button`, the fields, the dialog parts, `Spinner`, `Label`, `Card`, `CardTitle`,
  `Badge`, the menu and select items, and the three popups
  (`DropdownMenuContent`, `SelectContent`, `PopoverContent`) so far. Class constants that set
  no look are the exception: `DIALOG_SCROLL_*` (below), the motion constants
  (§1.5) and the shared focus rings (§1.7).
- **Every popup is drawn on one surface.** A menu, a select's list and a
  popover are `bg-popover text-popover-foreground` with the default border
  and `shadow-md`, set in the primitive and nowhere else. No call site picks
  a popup's surface: `bg-muted` there hid the highlighted item, since the
  highlight (`accent`) is the same color as `muted` in both themes. The
  theme test holds `popover` and `accent` apart for that reason. Width and
  padding are still the call site's (`className="w-80 p-0"`).
- **The small primitives take props for what call sites used to pass as
  classes**:

  | Primitive | Prop | Values |
  |---|---|---|
  | `Spinner` | `size` | `sm` 16px (default), `md` 20, `lg` 24, `xl` 32, `2xl` 40 |
  | `Spinner` | `tone` | `current` (default: the text color around it), `primary`, `success` |
  | `Label` | `tone` | `default`, `muted` (a field label under a section title of its own) |
  | `FieldHint`, `FieldError` | none | a field's help and its error, one size and colour each (§3.7) |
  | `Card` | `surface` | `muted` (default: the Settings panel), `raised` (the opaque card), `error`, `danger` |
  | `CardTitle` | `tone` | `default`, `destructive` |
  | `Badge` | `variant` | `default`, `outline`, `over-media` (a chip on a backdrop or poster, §1.2), `muted` and `tinted` (the fact and genre chips of a music hero) |
  | `Badge` | `size` | `sm`, `default`, `lg` |

  A spinner's tone is `!important` on purpose: inside an outline button,
  whose icons are forced to the button's color, it is what keeps the spinner
  in its own. `Card`'s `error` and `danger` surfaces differ only in border
  strength (20 % and 50 %), as the error card and the Danger zone are drawn
  today. A menu item is a pointer target by default, and a disabled select
  item is muted; neither needs a class.
- **`Badge`** (`ui/badge.tsx`) is the primitive for static pills — count
  pills, rating chips, status tags. It is non-interactive by design (a
  `span`); actionable chips are Buttons. The one rating chip is
  `TmdbScoreBadge` (§3.2). Pills that live in a semantic
  list nest the Badge inside the `<li>` (movie metadata chips, album
  metadata/genre pills). Navigational pills (e.g. the album page's artist
  pill) are `Link`s composing the pill classes with
  `FOCUS_VISIBLE_RING_CLASS` — deliberately not Badge, which stays
  non-interactive.
- **Narrow primitive contracts are intentional.** Alert is destructive-only;
  AlertDialog has one default size; dropdown items are non-inset; Select uses
  item-aligned content; Separator is horizontal and decorative. Add a branch only when a production surface requires it.
- **A dialog that can outgrow the viewport scrolls its body, not itself.**
  Technical Details and Edit Movie compose `DIALOG_SCROLL_CONTENT_CLASS` on
  `DialogContent` (a flex column capped at `100svh - 2rem`, no padding of its
  own), `DIALOG_SCROLL_HEADER_CLASS` on the `DialogHeader`, and
  `DIALOG_SCROLL_BODY_CLASS` on the one element that scrolls. The close
  button is positioned against the non-scrolling box, so it never rides the
  scrollbar track or scrolls out of reach, and a title the dialog focuses on
  open carries `outline-hidden` with no ring (§1.7). A body with no focusable
  content of its own (Technical Details) is itself a tab stop — `tabIndex={0}`
  with the shared focus ring — so a keyboard user can scroll it; a body full
  of inputs (Edit Movie) needs none.
- **When to add a variant vs. a constant**: a new *look* for an existing
  primitive (e.g. another Button treatment) → add an entry to its variant
  record, next to its siblings, and to the gallery (§2.5). A *cross-component* treatment (card chrome, motion, focus) → an
  exported constant in `lib/constants.ts`.
- All primitives stamp `data-slot="..."`; tests and cross-component behavior
  rely on these (e.g. `SELECT_CONTENT_SLOT_SELECTOR` keeps dialogs open while
  interacting with portaled selects). Keep them when customizing.
- **A select's list is always portalled to the body.** `SelectContent` takes
  no `container`: the list is positioned `fixed`, and inside an element with
  a transform (every centred dialog) `fixed` is measured from that element,
  so the list opens away from its field. A dialog that holds a select keeps
  itself open through `preventDialogDismissIfRadixSelectContent` instead.

### 1.7 Accessibility — non-negotiable

- **One focus recipe, in three prefixes.** `--ring` is glacier in both themes,
  and every focus indicator in the app is the
  `focus-visible:ring-[3px] ring-ring/50 border-ring` style. It reaches the
  page three ways, all exported from `constants.ts`:
  - `FOCUS_VISIBLE_RING_CLASS` — the default, for any inline control outside
    `components/ui`. The primitives ship the same string in their own base
    classes.
  - `CARD_FOCUS_WITHIN_RING_CLASS` — the whole-card variant, on a media card's
    `<article>`, so the card shows focus wherever it lands inside.
  - `PEER_FOCUS_VISIBLE_RING_CLASS` — for a rich `<Label>` standing in for an
    `sr-only` radio (`RadioCard`, §2.2), where the ring must follow the peer
    input's focus rather than the label's.

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

  A raw `<button>` that takes no recipe shows the browser's own outline,
  which reads as a different app; every one composes the shared ring. A
  scroll box clips a ring drawn outside its rows, so a list of focusable rows
  inside one gets `-m-1 p-1` on the scroll box (the Add to Playlist list):
  room for the ring, no change to the layout.
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
  rows resolve, and it stays out of sight because the cards, the error
  alert and the empty state already say the same where the eye lands.
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
- **Focus never ends on the page body.** A control that disables or removes
  itself while it holds focus hands focus on first, because a disabled or
  detached element cannot keep it and the next Tab would start from the top:
  - A button that disables itself (Save while it is pending and once the
    form is clean, Reset once it has reset) uses `useFocusHandoff`
    (`hooks/useFocusHandoff.ts`). The Settings Save bar hands focus to its
    status line and takes it back if Save comes back to life (a failed
    save); Account's "Save profile" hands it to the name field.
  - So does a button that is disabled only while its request runs: the
    field it saves takes focus and keeps it ("Update password", "Set URL",
    "Set PIN" and "Remove PIN", a device rename's Save, Quick Connect's
    Continue). Where there is no field, a heading takes it and gives it
    back once the button can be pressed again: the step's heading for Quick
    Connect's "Approve device", the library's heading for "Scan … library",
    which stays disabled for as long as its scan runs.
  - A control that saves on every change stays focusable while its save is
    in flight: `aria-disabled`, and a change is ignored until the save is
    back (`SwitchField busy`, the trailer selects, the avatar file input
    while it uploads). `disabled` dropped focus on each toggle, and a
    select's list or the file dialog could not hand focus back.
  - A text field is `readOnly`, not `disabled`, while the form it sits in
    saves (General, Libraries, Playback): Enter in the field is what
    submitted the form, so the field is what holds focus.
  - A dialog returns focus to what opened it, and when that is gone to the
    card's heading or its own action: a revoked device's row to "Devices"
    (`SettingsCardHeader titleRef`, `ConfirmDialog fallbackFocusRef`), a
    deleted user's row to "Add user". The row leaves the cached list with
    the request, not at the next refetch, so the fallback is what the dialog
    finds. An inline form (renaming a device) returns focus to its trigger
    on Save, Cancel and Escape, unless focus has already moved elsewhere.
  - A button that removes itself focuses what it acted on first ("Clear
    movies library path" focuses the path field).
  - A server answer that marks a field (a refused password change) focuses
    it only while focus is still in the form that sent the request: a
    reader who moved on keeps their place.
  - Not covered: a dialog's submit button while its request runs. Focus
    leaves it for that time, and the dialog's focus scope takes it back as
    soon as the button's content changes (the next Tab is the dialog's first
    field).
  - jsdom keeps `document.activeElement` on a disabled control, so a loss
    shows only in a real browser: check a new self-disabling control there.
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
    <span className="sr-only">Rated PG-13</span>
    <span aria-hidden="true">PG-13</span>
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
      only logged. Because the loader awaits that query, the route also sets its
      skeleton as `pendingComponent` (§3.4).
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
  + `@import "./animations.css"` (§1.5), declares `@custom-variant dark (&:is(.dark *))`
  (class-based dark mode), and maps every token into Tailwind via a
  generated `@theme inline` block (`--color-X: var(--X)`, §2.4). There is no
  `tailwind.config.js`.
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

### 2.2 The primitives in `components/ui`

- **These files are Igloo's.** They started as shadcn/ui "new-york"
  components and have since been trimmed, restyled and narrowed to what
  production uses. There is no `components.json` and no shadcn CLI: nothing
  is regenerated or pulled from upstream, and a snippet copied from the
  shadcn docs is adapted to the rules here (variants as records §1.6,
  `motion-reduce:` escapes §1.5, the shared focus ring §1.7, `data-slot`)
  before it lands.
- **Upgrades come from `radix-ui`.** Components import from the consolidated
  package (`import { Slot } from "radix-ui"`), not `@radix-ui/react-*`, and
  only files in `components/ui` import a Radix primitive (plus `ScrollRail`'s
  `Slot`). A Radix upgrade is therefore a change to that package and, at
  most, to these files, checked by the screenshot baselines (§2.5).
- Icons are lucide (§1.1).
- 26 primitives live in `components/ui`: alert, alert-dialog, avatar,
  badge, button, card, checkbox, dialog, dropdown-menu, input, label,
  native-select, pagination, popover, radio-card, select, separator, sheet,
  sidebar, skeleton, sonner, spinner, switch, tabs, textarea, tooltip — plus `sidebar-context.tsx`, which holds
  the sidebar's context so the primitive and `AppSidebar` can both read it,
  and `field-variants.ts`, the looks the field primitives share (§1.6).
- Three of them are Igloo's own, with no Radix part underneath:
  - `Switch` is a `role="switch"` button: `checked`, `onCheckedChange`, and
    a name from `aria-labelledby` or `aria-label`. `SwitchField` (§3.7) is
    the labelled Settings row built on it.
  - `RadioCard` is a radio drawn as a whole card. The native input stays in
    the tree, out of sight, so a group keeps the browser's arrow keys; the
    card is its label, so a click anywhere picks it. `footer` is content
    under the label that is not part of the radio's name (tie it in with
    `aria-describedby`); `as="li"` when the group is a list. The TMDB and
    Spotify pickers are its two users.
  - `Skeleton` is one muted box of a loading layout: size, and any radius
    other than the small default, from `className` (`rounded-none` for a
    square one). It has no role and does not pulse; the `SkeletonStatus`
    around the layout does both (§3.4). `pulse` is for a box that loads on
    its own.
- **`Tooltip` names a control whose meaning is an icon.** It wraps the
  control (`<Tooltip label="Switch to light theme"><Button …/></Tooltip>`)
  and shows on hover after 500ms and at once on keyboard focus; Escape
  closes it. Rules:
  - It never opens on touch, so it holds nothing a touch user needs. The
    control keeps its own `aria-label`; never a native `title`.
  - By default it repeats the control's name and is **not** announced as its
    description (Radix would read the name twice). `describes` is for a
    tooltip that adds something (why a control is disabled, a shortcut).
  - A control that explains why it is disabled must stay reachable:
    `aria-disabled` plus a guard in its handler, never `disabled`, which
    takes pointer events and (in a Radix menu) the arrow-key stop away, so
    the tooltip could never open. `DropdownMenuItem` dims an
    `aria-disabled` item like a disabled one (`RequestMediaMenuItem`, §3.2);
    its tooltip opens to the side (`side="left"`) so it does not cover the
    items above. Once the reason stops holding, the tooltip is switched off
    with `disabled`, not unmounted: swapping the wrapper away remounts the
    control and drops the focus resting on it.
  - It is drawn inverted (`bg-foreground text-background`) so it never
    reads as a menu, on the `z-popover` layer, at the micro duration.
  - `AppBoot` mounts one `TooltipProvider`, which lets the next tooltip show
    at once after the first; a `Tooltip` rendered outside it (a test, the
    gallery) brings its own. Inside a fullscreen player pass `container`.
- `NativeSelect` is the browser's own `<select>` styled as a field, for where
  the platform picker is the better control: touch devices (the playback
  settings pickers switch to it on a coarse pointer) and short filter lists
  (the person page). `Select` is the styled listbox.
- Their exported subcomponents are intentionally limited to production use.
  Portal, overlay, and Select scroll helpers stay private to their owning
  primitive. Sheet is the left mobile-navigation surface, and Sidebar exports
  only the provider, shell/inset, trigger/rail, and the header/content/footer,
  group, and menu pieces used by `AppSidebar`.
- **Change them in place**, and keep what the house rules depend on: the
  Button/Tabs variants (§1.6), `CardTitle`'s `asChild` heading support
  (§3.7), a `motion-reduce:` escape on every animated primitive,
  `spinner.tsx` consuming the shared motion constants, the fixed navigation
  contracts, and `sonner.tsx` (§3.6). A new variant or state is added to the
  gallery in the same change (§2.5).
- **No new dependencies** without explicit approval (project rule). Before
  adding a primitive, check whether an existing one + a constant covers it.

### 2.3 Shared constants: `web/src/lib/constants.ts`

One module holds every shared frontend constant — query keys, protocol
records, page sizes, TMDB image sizes, playback timings — and, in its second
half, the cross-component class strings that act as the de facto token layer
above Tailwind. Those are what this section is about; each carries a comment
explaining what it owns.

**The name is what gets a class string linted.** The Tailwind lint rules read
`className` attributes and `cn()` arguments, and otherwise find class strings
by variable name (`web/eslint.config.js`): a name ending in `_CLASS`,
`_CLASSES`, `_VARIANTS` or `_SIZES`, or in camel case `…Class`, `…Classes`,
`…ClassName` or `…Variants`. Keep to those endings for a class string or a
record of them, here and in a component file, and do not use them for
anything else: a record that mixes class strings with labels has its labels
reported as unknown classes. The order of classes inside a string is the
linter's and carries no meaning, so a test must not depend on it.

The styling families:

- **Cards** — `CARD_SURFACE_CLASS`, the media-card chrome: `group relative
  overflow-hidden rounded-xl border border-border bg-card`, hover lift +
  primary border + glacier glow, with the property-scoped 200ms transition and
  motion-reduce fallbacks embedded (via `CARD_INTERACTIVE_SURFACE_CLASS`,
  which stays exported for bespoke card shells that bring their own hover
  styles). Plus `CARD_MEDIA_HOVER_CLASS` (poster zoom),
  `CARD_OVERLAY_REVEAL_CLASS` / `CARD_ACTION_REVEAL_CLASS` (hover/focus-within
  overlay + scaled action reveal), `CARD_FOCUS_WITHIN_RING_CLASS` (§1.7) and
  `MEDIA_FALLBACK_SURFACE_CLASS`, the tile under a card or hero whose media
  is missing (§1.2). The chip over a poster or backdrop is the `Badge`'s
  `over-media` variant (§1.6).
- **Motion** — `MOTION_*`: page/section, media-overlay and -dialog,
  player-chrome, track-row and settings-surface classes, plus the
  loading/spinner states and the three `MOTION_DURATION_*_MS` numbers (§1.5).
- **Focus** — the three ring recipes (§1.7). `DETAIL_SECTION_HEADING_CLASS`,
  `DETAIL_RAIL_HEADING_CLASS`, `SKIP_LINK_CLASS`, `PLAYER_ICON_BUTTON_CLASS`
  and `PLAYER_PRIMARY_BUTTON_CLASS` all compose `FOCUS_VISIBLE_RING_CLASS`
  rather than restating it.
- **Page chrome** — `DETAIL_HERO_*` (hero shell, content, and the three
  literal over-media scrims), `MUSIC_DETAIL_*` (the music detail pages' shell
  over the backdrop band, hero row and Play/Shuffle actions row, shared with
  `MusicDetailSkeleton`, §3.2), `LIBRARY_TABS_LIST_CLASS` /
  `LIBRARY_TAB_TRIGGER_CLASS`, `LIBRARY_POSTER_GRID_CLASS` (the fixed-column
  library grid), `HOME_RAIL_POSTER_ITEM_CLASS` / `HOME_RAIL_ALBUM_ITEM_CLASS` (§3.2), `MINI_PLAYER_CLEARANCE_*` (the shell's
  reserved space under the mini bar, §3.1); Settings has no chrome constants
  of its own (its cards are the `Card`'s default surface, its fields the
  `inset` variant, its select lists the one popup surface and the save bar's
  Reset a plain `outline` button, §1.6),
  `PLAYER_FRAME_SHELL_CLASS` / `PLAYER_FRAME_CLASS` (the windowed player's
  frame, §3.5). Elevation for floating surfaces is the `shadow-floating`
  utility, not a constant (§1.4).
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
color as OKLCH + hex + comment (`THEME_TOKENS`), the per-theme elevation
(`ELEVATION_TOKENS`), the scales that are the same in both themes
(`SHADOW_SCALE`, `Z_INDEX_SCALE`, `MOTION_DURATIONS_MS`; §1.4, §1.5), the
boot-only colors (`BOOT_COLORS`), and `THEME_STORAGE_KEY`. `cn()` reads the
scale names from it too, so `tailwind-merge` resolves `shadow-floating`,
`z-modal` and `duration-micro` against Tailwind's own classes
(`src/test/lib/cn.test.ts`). Three files carry generated blocks
rendered from it by `scripts/generate-theme.ts` between
`BEGIN/END GENERATED` markers — **edit the module, run
`bun run generate:theme`, never hand-edit inside the markers**
(`--check` exits 1 if anything is stale):

1. `src/assets/styles.css` — two blocks: the `@theme inline` map that turns
   each token into Tailwind utilities (`--color-card: var(--card)`,
   `--shadow-floating`, `--z-index-modal`, `--transition-duration-micro`),
   and the `:root` / `.dark` token rules. A token added to the module gets
   its utility with no further step.
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
derives `THEME_COLORS` from the canvas tokens; it
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

### 2.5 Component gallery and screenshot baselines

The gallery (`/gallery`, `web/src/components/gallery/`) renders every
primitive in every variant and state, plus the overlays open
(`/gallery?open=dialog`, `alert-dialog`, `select`, `dropdown`, `popover`, `tooltip`). It
is the place to look at a primitive in both themes, and it is what the
screenshot baselines shoot. `/gallery?section=buttons` shows one section
alone, which is how the baselines shoot them: where a section sits on the
page changes a few of its anti-aliased pixels (3 to 103, measured), so on the
full page a section that grew would move every baseline below it.

- **Development only.** `src/routes/gallery.tsx` imports the gallery behind
  `import.meta.env.DEV`, so a production build drops its modules; the route
  itself stays (the generated route tree is the same in every mode) and
  redirects home. CI greps `web/dist` for the gallery's `igloo-ui-gallery`
  marker, and `e2e/boot.spec.ts` checks the redirect.
- **A new primitive, variant or state goes into the gallery in the same
  change**, in the section for its family (`GalleryPrimitives.tsx`) or as an
  overlay stage (`GalleryOverlays.tsx`). A new section also goes into
  `SECTIONS` in `GalleryPrimitives.tsx` and in `e2e/visual/gallery.visual.ts`,
  and into `src/test/gallery/gallery.test.tsx`.
- **"Call-site recipes" rows** show a primitive with the `className` overrides
  production passes today. When a recipe becomes a variant, switch the row to
  the variant: its baseline must not change.

**Baselines** live in `web/e2e/visual/__screenshots__/` and cover the gallery
(`gallery.visual.ts`) and a few whole pages on the mock API server
(`pages.visual.ts`), in both themes. They run from their own config,
`web/playwright.visual.config.ts`, always against the dev server
(`bun run test:visual`, `make test-visual`), and are a CI job (`test-visual`),
not part of `make check`.

- **The committed set is drawn by CI's pinned image**, and the comparison is
  exact: a single pixel that differs in any channel fails. It runs in two
  passes, both inside `expectScreenshot` (`e2e/visual/visual-helpers.ts`),
  which every visual spec uses instead of `toHaveScreenshot`:
  1. Playwright's `toHaveScreenshot` with `threshold: 0`, `maxDiffPixels: 0`.
     It owns the baseline files and the diff images of a failure, but its
     comparator skips a differing pixel it takes for anti-aliasing (measured:
     5–73% of the pixels a tint, radius, text-colour or shadow change moves).
  2. A pixel-for-pixel comparison of the same target against the baseline
     (`e2e/visual/png.ts`), with no such allowance. Its failure names the
     pixel count and the box that holds them. `comparator.visual.ts` checks
     the decoder against the browser's own decoding.
- **A mismatch is retried before it fails.** Chromium does not always
  rasterise one page to the same pixels: about one test in a hundred comes
  out with a few anti-aliased pixels of a rounded corner one shade apart. The
  exact pass shoots up to three times, and a comparing run retries a failed
  test twice on a fresh page, reporting it as flaky. A real change differs
  every time, so it still fails.
- A laptop renders text differently from CI, so a local run executes the
  specs without comparing pixels.
- **To prove a change is invisible locally**, draw a set on the base commit and
  compare the branch against it on the same machine:
  `VISUAL_STRICT=1 VISUAL_SNAPSHOT_DIR=/some/dir bun run test:visual:update`
  on the base, then the same without `:update` on the branch.
- **When a change is meant to be seen**, the `test-visual` job fails and
  uploads two artifacts: `playwright-report-visual` (expected, actual and
  diff images of what failed) and `visual-baselines` (the redrawn set). Review the
  images, then commit them over `web/e2e/visual/__screenshots__/` in a commit
  of their own, naming what moved and why.
- A Playwright upgrade or a new runner image can move pixels; regenerate the
  baselines the same way, in a commit that changes nothing else.

---

## 3. UI patterns

### 3.1 App shell

`AppShell.tsx`: skip link → `SidebarProvider` + `AppSidebar` (a fixed
left-side, icon-collapsible contract; 16rem expanded / 3rem icon rail / 18rem
mobile sheet that auto-closes on nav) + `SidebarInset` content column (a
`div`, not a `main`): the sticky app header inside it is the page's
banner landmark, and only the content area below it is `<main id="main">`, so
the skip link lands past the search bar. The
provider owns this state internally; the rail and Ctrl/Cmd+B toggle desktop
collapse, while the mobile trigger controls the sheet.

- Sidebar (`app/app-sidebar.tsx`), top to bottom:
  - **Header**: `BrandMark` on a `bg-primary` tile + "Igloo" wordmark linking
    home. It is `h-14`, the app header's height, so the two bottom borders
    meet in one line.
  - **Navigation** (one `navigation` landmark): Home; then Movies, TV Shows
    and Music under a "Library" heading that names their list; then
    Settings, pushed to the foot of the column (`mt-auto`). Active =
    `bg-sidebar-accent` + `text-primary` icon.
  - **Account block** (footer): the signed-in user's avatar (uploaded
    picture, else initials), name and role ("Administrator" / "Member"),
    opening a menu upward with their email, Account settings, the theme
    switch and Logout. Logout lives nowhere else.
  - **Collapsing**: the column collapses to its icons (3rem). The control is
    the `SidebarTrigger` at the left of the app header, named "Collapse
    sidebar" / "Expand sidebar" with `aria-expanded`; Ctrl/Cmd+B and the
    `SidebarRail` edge do the same. The state is kept in `localStorage`
    (`igloo-sidebar`), so it survives a reload. Collapsed, the heading and
    labels go, each icon names itself in a `Tooltip` to its right (§2.2),
    the account block is the avatar alone, and its menu opens to the right.
  - **Phone**: the same content in a sheet. The trigger is a 40px menu icon
    named "Open navigation".
- Header: sticky `h-14 bg-background/95 backdrop-blur-sm`, border-b.
  The `SidebarTrigger` at every width (above), then `app/Header.tsx` — a `role="search"` form
  (submits to `/search?q=…&tab=all&page=1`), `NotificationBell`,
  `ThemeToggle`. The search input is the `glass` field variant (§1.6), the
  light "frosted" treatment from `lib/input-styles.ts` (an allowlisted
  raw-color exception, §1.2).
- **Immersive routes** take the whole window. A route that sets
  `staticData: { immersive: true }` is rendered by the shell as the `main`
  landmark alone: no sidebar, no header, no page padding, at least as tall
  as the window. They are the two video players, the trailer and the watch
  room (`immersive-routes.test.ts` pins the list); each brings its own title
  and way back. A player fills the window exactly; the watch room is a page
  of cards, so its route adds the page padding back and it scrolls when it
  is taller than the window. Nothing moves in the route tree and no URL
  changes.
- Content: `px-4 py-6 sm:px-6 lg:px-8`; when the mini audio player is visible
  the shell adds `MINI_PLAYER_CLEARANCE_PADDING_CLASS` (`pb-28 sm:pb-24`) so
  content never hides behind it. The desktop sidebar column takes the same
  class, so its footer (the account block) sits above the full-width bar instead of under
  it; the mobile sheet opens above the bar and needs none. **Scrolling happens on the window** — virtual
  lists must use `useWindowVirtualizer` (§3.4), never a nested scroll
  container. For the same reason the content column clips with
  `overflow-x-clip`, not `overflow-x-hidden`/`auto`: those create a scroll
  container, and sticky children inside pages then stop sticking to the
  window.

**Unknown URLs** land on the shell's catch-all route (`routes/_auth/$.tsx`):
`MediaNotFound` with the heading "Page not found" and "Back to Home",
titled through `head` like every page, so a mistyped address keeps
the sidebar, the header and a way back instead of the router's bare text.

**The home page** is the shell's canonical composition: a billboard, then
six sections in order — `WatchRooms`, `ContinueWatching`, `LatestMovies`,
`LatestShows`, `LatestAlbums`, `MoviesInTheaters`.

- **The billboard** (`components/home/HomeBillboard.tsx`) leads with one
  title: the first Continue Watching item, or the newest movie when nothing
  is in progress, or nothing at all for an empty library. It shows the
  title's backdrop (its poster, blurred, when it has none; the fallback
  tile when it has neither or the image fails), an eyebrow saying why it
  is there ("Continue watching" / "New in your library"), the title as an
  `h2`, the year or the episode, the time left with a progress strip, the
  overview (two lines on a phone, three above), and two pill links: Resume
  or Play, and Details. The words are white over literal black scrims on
  the fixed tile, so they read the same in both themes (§1.2). On a phone
  the card is taller and its lower half nearly opaque, so the words sit
  under the picture rather than on it.
- The page's one `h1` is "Home", `sr-only`: the billboard leads with a
  title's name, which is not the page's.
- All sections but `WatchRooms` render through the shared `HomeMediaSection`:
  a heading, a "See all" link to the row's library where it has one, an
  `sr-only` described-by summary, and pending / error / empty states. Its
  cards sit in **one row that scrolls sideways** (a `ScrollRail`, §3.2),
  each card at a fixed width (`HOME_RAIL_POSTER_ITEM_CLASS` /
  `HOME_RAIL_ALBUM_ITEM_CLASS`), so the page is as tall on a phone as on a
  desktop.
- The route loader `ensureQueryData`s every one of these queries, so the
  page arrives complete rather than popping in section by section.

Each row's cards sit in a `<ul role="list">` named by the
row's heading (`aria-labelledby`), one `<li>` per card, so a screen reader names
the row when focus tabs into its first card — Orca + Firefox does not announce
the labelled region alone. A new home row is a `HomeMediaSection` with one of the
`HOME_RAIL_*_ITEM_CLASS` widths (§3.2) and its query added to the loader.

### 3.2 Media cards and library pages

- **A top-level page opens with `PageHeader`**
  (`components/shared/PageHeader.tsx`): an `icon` in the primary color, the
  `title` as the page's one `h1`, an optional one-line `description`, and the
  section entrance (§1.5). The Movies, TV Shows and Music libraries and both
  states of Search use it; a `title` may be markup ("Search results for
  ‘…’"). A library puts its counts (`meta`: the `LibraryStats` region) and
  its More menu (`actions`) on the title's own row. On a phone the counts
  take a line of their own under the title and the menu stays at the end of
  the title's line. Libraries carry no description
  line: the tabs follow the header directly, so the first row of posters
  sits about 90px higher than when the counts had a row of their own.
  Settings uses it too, with a description that names what the reader's own
  tabs hold: "Manage the server, your account and users." for an
  administrator, "Manage your account and playback." otherwise.

#### Poster cards

Every 2:3 poster card is `components/shared/PosterCard.tsx`, implemented once.
`MovieCard`, `ShowCard`, `InTheatersCard` and `ContinueWatchingEpisodeCard`
pass it their links, labels, badge and watch progress rather than repeating the
markup; a new poster card belongs there too. Reach for a fresh `<article>` only
when the anatomy genuinely differs — square covers (`AlbumCard`, and the
shared `PlaylistCard`, which serves music and movie playlists alike and takes
its link and placeholder icon from the playlist's `content_type`), circular
thumbs (`MusicianCard`), or the horizontal `WatchRoomCard`. All of them wear
`CARD_SURFACE_CLASS`.

A `PlaylistCard`'s square shows, in this order: the playlist's own
`cover_image`; a 2×2 collage when the list response's `artwork` has four
entries (album covers for music, posters for movies); the first entry alone
when it has one to three, since two or three would leave holes in the grid;
the placeholder icon. The images are decorative (`alt=""`), and one that fails
to load swaps the whole square for the placeholder: a collage with a hole
reads as broken.

```
<article class={cn(CARD_SURFACE_CLASS, CARD_FOCUS_WITHIN_RING_CLASS)}>
  <Link aria-label="Uncut Gems 2019" …>       ← whole-card link, full label
    <div class="aspect-2/3 bg-muted">              ← fixed-ratio box (no CLS)
      <img loading="lazy" decoding="async" fetchPriority="low"
           width={500} height={750} class="size-full object-cover" … />
      … onError → centered lucide icon on the media-fallback tile (usePosterFallback)
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
  {actions && <div class="absolute top-2 right-2 z-10 …">{actions}</div>}  ← card's own menu
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
- **The `actions` corner is for a control of the card's own**, a menu over
  the movie it shows, and it is the only interactive thing a card may carry
  besides its two links. It renders beside the poster link, never inside it
  (a button in an anchor is invalid and would navigate), reveals with the
  card's hover and focus-within like the play control, stays while its menu
  is open (`has-[[data-state=open]]`), and is always shown where nothing
  hovers (`pointer-coarse:`). Its trigger is an icon button on a dark round
  fill so it reads over any poster, named "More actions for <title>". The
  movie playlist page is the one user so far: `PlaylistMovieMenu` (Remove
  from Playlist) through `MovieCard`'s `actions` prop, only when the detail
  response says `can_edit`. Library grids carry no card menu; a movie joins
  playlists from its own page.
- Progress is always `WatchProgressBar`, and the percent comes from
  `watchProgressPercent` in `lib/format.ts` — the one definition shared by the
  bar and the `"N% watched"` in the card's link label, so the bar itself stays
  decorative.
- **Prefetch on hover**: the `onMouseEnter`/`onFocus` wiring lives in
  `PosterCard` behind an `onPrefetch` prop; each card supplies the query
  (`MovieCard` the movie details, `ShowCard` and `ContinueWatchingEpisodeCard`
  the show details). `InTheatersCard` passes none — an unreleased title has no
  detail query to warm.
- A rating is one thing everywhere: the labelled `TmdbScoreBadge` ("TMDB
  8.2", `sr-only` "TMDB user score: 8.2 out of 10"). The library movie's
  `critic_rating` is TMDB's `vote_average` from the scanner, so the movie hero
  shows it as that badge, like the show and in-theaters heroes;
  `InTheatersCard` wears the `size="sm"` badge in its poster corner,
  `aria-hidden` because the card's link label already speaks the score. There
  are no tiered amber/glacier rating chips (`lib/rating.ts` is gone), and
  `audience_rating` has no source on the server, so nothing renders it.

#### Grids and rails

The canonical library poster grid is `LIBRARY_POSTER_GRID_CLASS`
(`grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6`). Home
sections are rails instead (below): their cards take a fixed width per
breakpoint (`HOME_RAIL_POSTER_ITEM_CLASS`, `HOME_RAIL_ALBUM_ITEM_CLASS`), so
a sparse row cannot stretch one poster across the content column; pinned by
`constants-contracts.test.ts`.

True horizontal rails (the home rows, cast, chapters, extras, the seasons tab strip) are
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
`Library*`: `LibraryStats` (the labelled count region in the page header's
`meta` slot; it takes `figures`, one or several, and joins them into a single
`Library statistics: 42 movies` / `…: 3 albums, 40 tracks, 2 musicians`
name. From `sm` it sits on the title's row between the title and the More
menu; on a phone it takes the line under the title, where its figures may
wrap, while "More options" stays at the right end of the title's line; the
`PageHeader` note that opens this section has the layout), `LibraryMoreMenu` with `RefreshLibraryMenuItem` and
`RequestMediaMenuItem` (the "More options" dropdown; every library has at
least Refresh Library, which refetches the page's cached queries and toasts —
Movies adds Request Movie and Music adds Request Album / Request Track beside
it, each a `RequestMediaMenuItem` that stays disabled (`aria-disabled`, still
in the arrow-key order) until its provider's status answers and states the
reason in its accessible name, its tooltip (beside the menu, on focus and
hover) and an `sr-only` tail, since a disabled menu item otherwise announces
only its label; the page owns the status query, because one mounted inside the dropdown
would not fetch until the menu opened), `LibraryAllTab` (the paginated grid and its skeleton),
`LibraryGenresTab` (the genre-tile facet with focus restoration after Clear,
and its skeleton), `LibrarySortToggle` (the single A–Z / Z–A button) and
`LibraryEmptyState` (the minimal empty variant, §3.4). A page supplies what
differs: the card renderer, the lowercase nouns for copy and announcements, its
prepared `queryOptions()`, the tab triggers, and navigation callbacks — the
page keeps the typed `navigate({ to, search })`, so the shared tabs never
learn a route. Navigations spread the previous search (`prev => ({ ...prev,
tab })`) and the route's `stripSearchParams` middleware drops whatever equals
a default, so a tab, page or sort at its default never appears in the address
bar; the liked view is `playlistsView=liked` on both library pages. `LibraryAllTab`'s `sort`/`onSortToggle` pair is optional: an
API that sorts (movies, shows) passes both, one that does not (albums,
musicians) passes neither and gets no toggle. The toggle renders while the tab
loads (a URL-backed sort works before the page lands) and, once settled, only
when the list has at least two items to order — an empty or single-item list
and a failed load hide it. Its `gridClassName` and
`skeletonCard` swap the 2:3 poster grid for another card geometry — the Albums
tab keeps the poster grid with `AlbumCardSkeleton`, the Musicians tab passes
its five-column round-thumb grid with `MusicianCardSkeleton`. The tab owns one
toolbar — page info on the right, then the sort toggle, with `toolbarStartSlot`
for anything a page puts on the left — and renders it identically while
loading and loaded, reserving its height while loading so the grid does not
shift down when the data lands (§3.4). A tab with no start slot drops the row
once it knows it has a single page, nothing, or an error — a failed refetch
included, whose stale page count no longer applies — rather than holding an
empty band above the grid; the sort toggle hides in the same cases, so it
never holds the row open by itself, and that one settle is the only time such
a tab's grid moves. The liked-movies and liked-tracks views
are each a `LibraryAllTab` whose `toolbarStartSlot` carries the "Back to
playlists" control and count, so they inherit the tab's out-of-range page
clamp and keep that control reachable while loading, empty and errored. The
liked-tracks view passes `TRACK_LIST_CONTAINER_CLASS` as its grid and a
`TrackRowSkeleton` (the Tracks tab's placeholder row) as its skeleton card. The movie playlist page is
the same shape: its header above a `LibraryAllTab` whose `toolbarStartSlot`
names the list and whose `emptyMessage` scopes the empty copy to the playlist
rather than the library. The Playlists tabs themselves stay local to their
pages, as the Tracks tab does to the music page, but share their parts: one
`PlaylistsTabToolbar` (count on the left; the liked view and "New playlist"
pills on the right) that renders while loading and on error too, with the
count left out until the list lands, and a `PlaylistCardSkeleton` authored
beside `PlaylistCard`. The Tracks tab's A–Z letter rows are visual index
marks (`role="presentation"`, `aria-hidden`) inside its `role="list"`, whose
owned children must all be list items; each track row keeps its
`aria-posinset`/`aria-setsize`, so a screen reader still hears where it is,
and the first row of each letter opens with a visually hidden cue ("A
section", "Other titles section" for `#`) so the start of a letter is
announced where the sighted reader sees the mark.
Leaving the liked view returns focus to its toolbar
button, and entering it focuses "Back to playlists". Both playlist pages
share their owner row too: `PlaylistOwnerActions` (Edit and Delete text
buttons, rendered only when the detail response says `is_owner`; when it does
not, the header names the owner instead — "By {owner.name}" as a third item in
the music page's statistics list and a line under the movie page's count, the
way a watch-room card says "Hosted by" — so a public or shared playlist opened
by link never reads as nobody's) and the
`PlaylistFormDialog` it opens, which is the same dialog the toolbars create
with, keyed by playlist kind (`lib/playlist-kinds.ts` holds each library's
client calls, query keys, route id and Playlists-tab search). Edit resends
the cover and visibility it does not show, because PUT replaces every
field; Delete confirms through `ConfirmDialog`, then returns to that
library's Playlists tab. Items join a playlist of either kind through the
shared `AddToPlaylistDialog` (`components/shared`), keyed the same way: the
viewer's editable playlists of that kind with their counts, multi-selected,
one request per playlist. The server reports a duplicate as skipped rather
than failing, so the dialog only learns "already in" from the counts
afterwards; a refused request (an error envelope) is told apart from a
skip and toasts through `showActionFailed`, with the dialog left open.
Leaving a playlist asks nothing (it is reversible) and toasts
`showRemoved`: a track from its row menu, a movie from its card menu, both
only when the detail response says `can_edit`. A removed card takes its
menu's trigger with it, in either order: the card menu's close falls back
to the page `h1` (through `focusDialogRestoreTarget`) when its trigger is
disabled while removal is pending or already gone. If removal fails, focus
stays on the heading after the trigger becomes enabled again. Dismissing the
menu without removing returns focus to its enabled trigger. Once the list
has refetched, the movie playlist page moves a focus that fell to the document
onto that same `h1` (a `tabIndex=-1` skip-link target like the music page's).
A new library page composes the
same parts. The
search page's category tabs reuse `LIBRARY_POSTER_GRID_CLASS`,
`LoadErrorAlert` and `PosterCardSkeleton` but stay page-local: their result
count line, "No albums match 'q'" copy and non-grid tracks list are not a
library tab.

**Genre tiles.** `LibraryGenresTab`'s facet is a grid of media tiles, one
geometry whether or not a genre is selected: a backdrop from the genre
(`backdrop_path` on the genre list, which the server picks so that tiles show
different titles) fills the tile, and the name and count sit in a band along
the bottom. The band is opaque behind the text (`bg-black/70`, white name,
`text-white/80` count: §1.2's over-media exception) and fades out above it, so
the text keeps its contrast on any image and wraps to two lines without
leaving the band. A genre without a backdrop, or whose image fails, shows
`MEDIA_FALLBACK_SURFACE_CLASS` under the same band, so tiles look alike in
both themes. Pressed (`aria-pressed`), the band turns `bg-primary` with
`text-primary-foreground` and the image stays above it; a see-through primary
tint over the whole image computes below AA in light on a bright
backdrop. Tiles are `h-24` (`h-20` on phones, where three columns keep a
selected genre's results within reach), three to seven per row by the room
left beside the sidebar. The tile is a raw `<button>` with its own classes:
its whole surface is the control, as a card's is its link, and it takes no
`Button` look. Its "Clear" control is a `Button` (`outline`, `sm`, pill).

#### Detail pages

Movie, show and in-theaters detail pages are built from shared parts —
`DetailHero` (+ `DetailTitleHeading`, `DetailGenresList`, `DetailBackdrop`),
`DetailSkipLinks`, `DetailSkeleton`, `OverviewSection`,
`AboutSection`/`AboutRow`, `CrewDisclosure`, `TmdbScoreBadge` — with each
media type supplying only its own metadata chips, key-crew summary, and about
rows.
Adding a media type means supplying those three, not building a fourth page.

The hero is full-bleed on three sides: `DETAIL_HERO_SHELL_CLASS` cancels the
shell's page padding at both sides and at the top (`-mt-6` against the
shell's `py-6`), so the backdrop starts at the app header's bottom edge with
no strip of page background between them. `DetailSkeleton` uses the same
shell, so the loading page does not jump. Below `lg` the poster, title, chips
and actions stack over the whole backdrop, so `DETAIL_HERO_SCRIM_BOTTOM_CLASS`
covers the full height there and is darker (`from-black/95 via-black/70
to-black/30`); from `lg` the words keep to the lower left, the bottom scrim
goes back to the lower three-fifths and the side scrim carries them.

The crew section is headed "Key Crew" only when it bills someone above the
full list: a movie's director or writers, a show's creators. When it holds
only the "Show all crew" disclosure it is headed "Crew" and its skip link
reads "Skip to crew". Both words come from `crewSectionLabels`
(`lib/crew.ts`) so the link and the heading it jumps to never disagree; the
heading keeps `id="crew-heading"` either way.

Person profiles (`/people/$id`, using the TMDB person ID) have a portrait-led
header: a rounded 2:3 portrait beside the name and biography at desktop widths,
stacked on mobile. The main portrait uses the same-origin `original` image
proxy; additional portraits use `w185` thumbnails in `ScrollRail`, with labelled
links to full-size proxied images. All portraits have the shared image fallback.
Biography paragraphs are preserved; text longer than 600 characters has an
in-place “Read more / Show less” button with `aria-expanded` and `aria-controls`.
Missing biographies have explicit copy; missing personal facts and photo rails
are omitted. Personal facts use a description list, with calendar-based age or
age at death when the dates are valid.

Cast cards, key and expanded crew names, and series creators are typed person
links. Credit row keys stay separate from person identity. A validated
`returnTo` search parameter preserves the originating movie, in-theaters or TV
show detail URL (including season selection); direct visits return to Home.
Person pages share `MediaDetailGuard`, section skip links, focus and motion
recipes, and a portrait/biography/credit-row skeleton for pending navigation.

“Known for” shows up to eight shared poster cards, preferring the known-for
department then popularity. Filmography groups all distinct roles per movie or
show and uses chronological rows, newest first, undated last. Media, work and
library filters live in the URL, reset pagination on change, and show 24 titles
per page through `LibraryPagination`. Results changes are announced. Available
titles have a textual “In your library” label and open their Igloo detail page;
other titles open TMDB in a new tab. External profile and photo links name the
destination and new-tab behavior. Stored names, portraits and library credits
remain visible when TMDB is unconfigured, unavailable or no longer knows the
person; the partial-data notice distinguishes those states and offers the
shared retry control for a temporary failure.

`DetailTitleHeading` keeps a text-node space between the title and the
parenthesised year, since the two are flex items and would otherwise be read
as one word. `CrewDisclosure` expands the full crew in place as a one-, two-
or three-column grid — the window is the page's only scroller (§3.1), so no
section opens a nested scroll area.

The movie hero's `actionsSlot` (`MovieDetailsHeroActions`) is a 2×2 grid
below `sm` — Play, Watch, Like, and the More menu with a visible "More" label
beside its icon — and a wrapping row from `sm` up. The More menu lists
Playback Settings, Watch Together, Add to Playlist, Edit (admins),
Technical Details, then Delete (admins) after a separator; every dialog it
opens is lazy and returns focus to the trigger. An action row that relies
on `flex-wrap` with content that cannot shrink drops its last control onto
a lonely row at 390 px; a grid never does.

The album and musician pages keep their own hero anatomy — a square cover or
a round thumb beside the title, over a decorative 21:9 band, rather than
`DetailHero`'s poster — and share the music-specific parts in
`components/music/`: `MusicDetailBackdrop` (the aria-hidden band, on
`usePosterFallback`; its image, its fallback and the exported
`MusicDetailBackdropSkeleton` share one band height and, like the detail
hero, reach the app header with `-mt-6`; with no artwork the band is a plain
`bg-muted` box with no icon, because the cover or photo beside the title
already shows the labelled placeholder), `MusicDetailArt`
(`variant="album" | "musician" | "playlist"`: the square cover or round photo
beside the title on all three pages, with a labelled placeholder when there is
no artwork, and the `MusicDetailArtSkeleton` that reads the same sizes),
`MusicStatList` (the album and musician stat pills — `MusicStatChip` per
figure, `MusicDurationChip` speaking "Total duration …" through an `sr-only`
span) and `MusicGenreList` (their genre pills), `PlayShuffleButtons` (the
`lg` Play pill and outline Shuffle on all three pages, each page passing its
own accessible names; never disabled, a spinner while a playlist's remaining
pages load) in a `MUSIC_DETAIL_ACTIONS_CLASS` row, `MusicDetailSkeleton`
(`variant="album" | "musician" | "playlist"`, built from the same
`MUSIC_DETAIL_*` rows, `MusicDetailArtSkeleton` and
`MusicDetailBackdropSkeleton` as the pages: the album and musician variants
are one hero geometry with the art shape and the order of the pill rows
switched, and `withMenu` holds a place for the album's admin "More options"
button only when the page will show one; the playlist variant mirrors the
playlist page instead, which has no backdrop band and no overlap, just a
square cover beside the title over the rows), and
`MusicDetailBackNav` (the `nav` "Page navigation" landmark back to the owning
`/music` tab, also used by the playlist page). All three pages, the playlist
included, guard through `MediaDetailGuard` (§3.4), so a malformed id, a failed
load and a missing payload each render `MediaNotFound` with a link back to the
owning tab, never a bare heading or a lone spinner.

A catalog match shows up in three places, each absent when there is no
match. An album is matched on Apple Music or on Spotify, never both; a
musician only ever on Spotify. `CatalogLink` ("Open on Spotify" with the
`SpotifyGlyph` in the brand green, or "Open in Apple Music" with the Apple
Music mark in `text-foreground`, beside underlined primary text) opens the
matched album's or artist's page in a new tab and, like the people page's
external links, carries the destination and new-tab behaviour in its
accessible name ("Blue Record on Spotify (opens in a new tab)", "Blue Record
on Apple Music (opens in a new tab)"); the album page keeps it as an "Apple
Music" or "Spotify" row in the Album Details list, the musician page under
its stat pills. A single or compilation, as whichever catalog matched the
album reports it, earns one more `MusicStatChip` ("Single", "Compilation") at
the head of the album's stat pills; a plain album gets no label. An explicit
track, flagged by the catalog that matched its album, wears a small outline
`Badge` after its title in `TrackItem`: a visible "E" that is `aria-hidden`,
with "Explicit" in an `sr-only` span, so the row reads as "Alabaster
Explicit" and never as a bare letter.

#### Seasons and episode rows

A long control strip scrolls on the rail bleed rather than wrapping.
`ShowSeasonsSection` is a full `Tabs` pair — a `TabsList` of fully
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
  `lib/constants.ts` — `w500` posters, episode stills and genre-tile backdrops, `w1280`
  backdrops, `w185` profiles, `w92` network logos; the proxy accepts only those plus
  `original`. A value that is already a server URL (`/api/static/...`, a copy
  the server downloaded while "Download images" is on) comes back unchanged:
  the copy exists at one size. Music images go through `getMediaImageUrl()`
  (`lib/media-image-url.ts`). Never hit `image.tmdb.org` directly from the
  client.
- **Standard `<img>` recipe**: `loading="lazy" decoding="async"
  fetchPriority="low"`, explicit `width`/`height`, `object-cover`, inside an
  aspect-ratio `bg-muted` box. Add responsive `sizes` on dense grids
  (see `AlbumCard`). Only the login backdrop uses
  `loading="eager"`/`fetchPriority="high"`. A detail page's own hero art —
  `DetailHero`'s poster, `MusicDetailArt`'s cover or photo — is that page's
  largest above-the-fold image, so it keeps the explicit `width`/`height` but
  drops `loading="lazy"`/`fetchPriority="low"`.
- **Fallbacks, never broken images**: `onError` swaps to a centered muted
  lucide icon (Film / Disc / User) via the `usePosterFallback` hook (tracks
  the failed URL so a changed URL retries). Verified live on covers with
  missing art. Where white over-media text sits on the box (poster cards,
  the detail hero), the icon sits on `MEDIA_FALLBACK_SURFACE_CLASS` (§1.2).
- **Alt policy**: decorative images (a poster inside a link that already has
  an `aria-label`, backdrops) get `alt=""`/`aria-hidden`; informative images
  get descriptive alt (`Album cover for {title}`, `Photo of {name}`,
  `Playlist cover for {name}`).

- **Network logos** (`TMDB_LOGO_SIZE` `w92`, mostly dark marks drawn for
  white backgrounds) sit on a `rounded-sm px-1 py-0.5 dark:bg-white/90` plate
  in `ShowAboutSection`, so they survive the dark theme; the padding is always
  on so a theme switch moves nothing, and the network's name is always printed
  beside the mark, which stays `alt=""`.

### 3.4 UI states — loading, empty, error

- **Loading, two tiers.** Route navigations suspend in loaders
  (`ensureQueryData` + `defaultPreload: "intent"`). Once a loader outlasts
  the router's `pendingMs`, the pending view renders **inside the shell's
  content area**, so the sidebar and header stay put. A detail route shows
  its own skeleton: it lifts the skeleton it hands `MediaDetailGuard` into
  one route-local component and also sets it as `pendingComponent`. Every
  other route gets `defaultPendingComponent: RoutePending` (`src/App.tsx`,
  a `role="status"` spinner and "Loading..."). Only boot, which waits in the
  `_auth` layout before the shell exists, shows the full-screen
  `AppLoadingScreen` ("Starting Igloo..."); that layout sets it as its own
  `pendingComponent`. An **in-page navigation never waits in the loader**:
  a tab, page, sort, genre, search or season change re-runs the same route's
  loader with `cause: "stay"`. `loadOnEntry` (`src/lib/route-loads.ts`) then
  only starts the queries, so the URL change commits at once and the page's
  own skeleton covers the wait. Entering or preloading the route still awaits
  them, so a fresh page arrives complete. Within a page, each query renders a **skeleton that
  matches the real layout's grid geometry exactly** (same columns, same aspect
  boxes) so content arrival causes no layout shift — see the shared
  `DetailSkeleton` (`withActions` mirrors whether the real hero has an actions
  row; pages append their own below-the-fold geometry as children),
  `MusicDetailSkeleton` (§3.2; `withMenu` likewise mirrors the album's admin
  button), and the `LibraryAllTabSkeleton` /
  `LibraryGenresTabSkeleton` that live in the same files as the grids they
  mirror. A skeleton mirrors the **grid only**; chrome whose presence depends
  on the response — a library tab's page info — belongs to the component that
  renders it in every state, never to the skeleton, which cannot know how many
  pages there are before the query resolves. A grid skeleton repeats one card placeholder that lives beside the
  card it mirrors — `PosterCardSkeleton` in `PosterCard.tsx`,
  `AlbumCardSkeleton` and `MusicianCardSkeleton` in their card files — so a
  card and its placeholder change together. Skeleton geometry is authored
  directly in each loading layout with muted boxes (on the detail hero's
  muted band the placeholders are `bg-foreground/10`, a tint that shows
  against the band in both themes where `bg-background/50` vanished in
  light) and the shared
  `MOTION_LOADING_STATE_CLASS`, always beside the layout it must mirror — a
  skeleton moves with its layout, never on its own. `ui/spinner.tsx` (`role="status"`) uses
  `MOTION_SPINNER_STATE_CLASS`. Skeleton layouts hide their visuals with
  `aria-hidden` under a single `role="status"` + `sr-only` label: every
  skeleton renders through `SkeletonStatus`
  (`components/shared/SkeletonStatus.tsx`), which takes the spoken label
  ("Loading playlists") and, for a skeleton that pulses as one block, the
  `MOTION_LOADING_STATE_CLASS` as its `className`. The boxes inside are
  `Skeleton`s (`ui/skeleton.tsx`, §2.2).
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
    `refetch()`). The control is the shared `RetryButton`; every load-failure
    surface renders that same button, never its own.
  - A section that failed as a whole → `SectionErrorAlert`, the same
    destructive tint as an `Alert` with an optional title ("Error" by
    default): a home section's query (rendered for you by
    `HomeMediaSection`), the watch-room list. Both hand the query's `refetch`
    in as `onRetry`, so the alert carries the same Try again control as
    `LoadErrorAlert`.
  - A later page of an infinite list (the Tracks tab, a playlist's tracks) →
    `InfiniteListFooter`. The pages already shown stay, and a
    `LoadErrorAlert` under them retries the page (`fetchNextPage()`);
    scrolling back to the end retries too. Only a list with nothing loaded
    yet fails as a whole. These pages throw rather than cache an envelope, so
    their detection is `isError && !data`.
  - A Settings card → `SettingsErrorCard` (and `SettingsLoadingCard` for its
    pending state), so the card keeps its place in the page.
  - The four ways a detail page fails to show its subject — an invalid id, a
    failed request, one still in flight, an empty response — are
    `MediaDetailGuard`, which every `$id` route wraps its content in. Three of
    the four are dead ends, so each renders `MediaNotFound` with a page `h1`
    ("Movie not found" for a bad id, a 404 or an empty response, "No access"
    for a 403, "Couldn’t load this movie" otherwise); only that last, generic
    branch offers Try again — it is the only failure a retry can fix — and
    each `$id` route passes its query's `refetch` as `onRetry`; its destination is
    a named key (`music`, `moviePlaylists`, …) carrying both the route and the
    words on the link, so the two can never disagree and a destination may
    carry search params.
  - A missing resource → `MediaNotFound`: the page `h1`, an untitled
    `SectionErrorAlert` with one sentence (the heading already names the
    failure), Try again inside that alert only when the caller passes
    `onRetry`, and a **required** "Back to Movies/TV Shows/Music/Home"
    outline link, so the page never dead-ends.
  - The shell's own auth check gets no answer (offline, the server down, a
    proxy error) → `AppLoadError` (`components/app/AppLoadingScreen.tsx`),
    the `_auth` layout's `errorComponent`: `AppLoadingScreen`'s frame with an
    `h1` "Igloo", a `role="alert"` "Couldn’t load Igloo." and the shared
    `RetryButton`, which re-runs the check (`router.invalidate()`). Only a
    401 means signed out, so a failure never sends a signed-in reader to log
    in. No route inside the shell has its own boundary, so the same card
    catches a page that fails to render ("Couldn’t load this page.").
  - The session ends mid-visit (expired, revoked, the user deleted) → a 401
    from any request whose caller does not answer one itself
    (`handlesUnauthorized`: login, logout, the current password or PIN)
    signs the reader out through `useEndSession`: playback stops, an info
    toast says "You've been signed out.", and the login page returns to the
    page they were on.
  - A mutation → **toast** via `toast-helpers.ts`, never inline.
  - **The UI owns the words for failures it can name.** `apiRequest` stamps the
    HTTP `status` on every failure envelope it returns, and a surface maps the
    statuses it understands to a sentence — a 404 on a detail page reads "We
    couldn't find that movie.", a 403 "You don't have access to this
    playlist.", a 400/401 on login "The email or password is incorrect.", a
    403/404 on a watch room "This room no longer exists or you were not
    invited." A page about one subject (`MediaDetailGuard`, the trailer
    dialog) then uses a fixed sentence of its own for everything else
    ("Something went wrong while loading this movie.")
    and shows no server text at all. List and section loads go through
    `apiErrorMessage`, which keeps the server's message only for a 4xx other
    than 404 and otherwise uses the surface's own sentence, so a 404 (whose
    body `apiRequest` replaces with a canned string) or a 5xx (internal error
    text) never reaches the screen. Never show the client's canned "404 - The
    resource…" or "500 - A network error…" strings or a lowercase server
    constant as the copy.
  - **Failure copy names what did not load and stops.** A fallback never
    guesses at a cause ("Check your connection…") or prescribes timing
    ("Please try again later"): `apiRequest` cannot tell an offline reader
    from a server that is down or a proxy that answered HTML, and the Try
    again control beside the sentence already carries the action. Fallbacks
    read "Couldn’t load movies.", "Couldn’t load watch rooms.", "Couldn’t run
    that search.".

  Because the erroring subtree often unmounts its own live region, error
  surfaces carry `role="alert"` and announce themselves — never repeat one
  through `LiveAnnouncer` as well.

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
  frame paints, so the page never opens on a black box; while neither the
  movie surface nor the pre-roll is mounted, because the resume offer is
  still undecided, a placeholder in the player's geometry —
  `PLAYER_FRAME_SHELL_CLASS` / `PLAYER_FRAME_CLASS`, shared with
  `VideoPlayer` — shows that poster on black, so the dialog never floats
  over a bare box). It renders
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
  `chromeFullscreenMode`) the container goes `fixed inset-0 z-modal`, chrome
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
  Space/K, **N** skip, **S** start movie, F, Escape. If the browser blocks the
  autoplay (a cold direct load of `/movies/{id}/play` carries no user
  activation), the hook's `muteOnAutoplayBlocked` answers YouTube's
  `onAutoplayBlocked` by muting and playing again — muted autoplay is always
  allowed, and the retry happens once per player — and the chrome gains an
  outline **Unmute** ("Unmute trailer", **M**) beside the play control, which
  unmutes, removes itself and hands focus to Skip trailer; a `LiveAnnouncer`
  says the trailer is playing muted. When autoplay is allowed nothing
  changes. Each trailer is announced once ("Trailer 1 of 2: Title") through
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
  "Track N of M"; the like heart sits on the title line, centred on it behind
  a mirrored `size-10` spacer so the title stays centred, with the artist line
  beneath on its own); "Minimize player (Escape)" (no suffix on touch, §1.7)
  collapses it to the **docked
  mini bar** (`fixed inset-x-0 bottom-0 z-chrome bg-background/95 backdrop-blur`)
  with track info, transport, close, and a bottom progress strip; below `sm`
  the like and close buttons give way so the title keeps room to read (both
  stay reachable from the fullscreen view). Every track row carries the
  `TrackActionsMenu` (Add to Playlist, which opens the shared
  `AddToPlaylistDialog` of §3.2, then the links that lead somewhere
  else: an album row offers "Go to Artist", a musician row "Go to Album"),
  so a track can join a playlist from wherever it is listed. Below `sm` a
  track row moves its duration under the title — onto the subtitle line after
  a `·`, or alone when the row has no subtitle — and tightens to `gap-2`, so
  the title keeps room beside the heart, menu, play and (on a playlist) the
  drag handle, all of which stay. The right-hand duration is `hidden
  sm:inline` and the inline one `sm:hidden`: exactly one is in the
  accessibility tree at any width, never an `sr-only` copy, and the virtual
  row height (`VIRTUAL_LIST_TRACK_HEIGHT`) is unchanged because the second
  line already existed. A row outside the album page starts with its album
  cover: a 40 px `aria-hidden` tile (`coverUrl`, filled by `trackRowProps`
  from `album_cover`; the standard lazy `<img>` recipe on `usePosterFallback`,
  a muted disc when there is no cover so the titles stay aligned). 40 px sits
  inside the two text lines, so the row height holds; below `sm` the tile is
  hidden, because the title needs that room. The album page passes no
  `coverUrl`: every row would repeat the cover above the list, and it keeps
  its track numbers. A row's play button is always present but quiet — an
  unfilled, muted icon — and takes the filled primary look on row hover
  (`group-hover:`), on its own `focus-visible`, on the current track, and on
  every row where nothing hovers (`pointer-coarse:`). It is never hidden, so
  no action sits behind hover alone (§1.7). The bar
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
  (`showActionFailed(...)` etc.), not ad-hoc `toast()` calls: importing
  `sonner` anywhere but that file and `ui/sonner.tsx` is a lint error. Hand
  `showActionFailed` the failed envelope, not its `message`: it words the
  description through `apiErrorMessage`, so only a server-worded 4xx reaches
  the reader and a 5xx constant or the client's canned 404/network string
  never does. Inline error text and load-error cards make the same choice
  with `apiErrorMessage(res, fallback)`. `showInfo` carries neutral news
  that no action of the reader's caused: the session-expiry sign-out (§3.4)
  app-wide, and the player saying full screen is not available.
- **NotificationBell** (header): ghost icon button with a glacier unread
  badge pill ("99+" cap; count also in the `aria-label`), opening a `w-80`
  popover — header row with "Mark all read", `max-h-96` scroll body
  with spinner / a compact `LibraryEmptyState` ("You're all caught up." under
  a faded `BellOff`) / `divide-y` list states. Unread
  rows tint `bg-muted/40` with a glacier dot; rows show type label, message,
  relative time, per-row dismiss. A row is one raw `<button>` (its whole
  surface opens the notification) with the shared focus ring; dismiss is a
  `Button` (`ghost-muted`, `icon-sm`). Unread count polls every 30s; the list
  query is `enabled` only while open.

### 3.7 Forms & inputs

- Settings pages are Cards on their default `muted` surface
  (`border-border/50 bg-muted/30`, §1.6) with a `SettingsCardHeader` (glacier lucide
  icon + title + description), labeled fields in the `inset` variant at
  `size="lg"` (§1.6), and per-field or
  per-card Save buttons; destructive areas get the tinted "Danger zone" card
  (`border-destructive/*` + destructive text + outline destructive actions).
  A card's own loading and error states are `SettingsLoadingCard` and
  `SettingsErrorCard` (§3.4), so the card keeps its place in the page.
- **Every Settings tab is one `SettingsPage`** (`settings/SettingsPage.tsx`):
  a `max-w-5xl` column with `space-y-6` between cards, and the scroll margin
  that keeps a focused control clear of a sticky Save bar. A tab that is one
  form saved by a Save bar is `<SettingsPage as="form">` (General,
  Libraries). No tab sets its own width or gap; a card may cap its content
  (`CardContent className="max-w-2xl"` on Account, where the fields are
  short). `SettingsLoadingCard` and `SettingsErrorCard` are the same width.
- **`SettingsCardHeader` is the header of every Settings card.** `action`
  puts the card's own button at the end of the header's row (Users: "Add
  user"), stacked under the title on a phone. `tone="destructive"` draws the
  Danger zone's title, icon and description in red. No card builds a header
  from `CardHeader` by hand.
- **A removal outside a confirmation is `outline-destructive`**: "Delete
  account" in the Danger zone, "Remove PIN", "Revoke". The filled
  `destructive` button is the confirming one, inside the dialog.
- **A submit button shows a `Spinner` before its pending label** ("Saving...",
  "Creating..."), in a card, a dialog or the Save bar alike.
- **A role or status chip is a `Badge`** (Users: `tinted` for an
  administrator, `outline` for a user), and a list that is still loading
  says so in a `role="status"` line with a spinner.
- **A field has one label, and its help sits above the control.** The order
  is label, help, control, then whatever depends on the value:

  ```tsx
  <Label htmlFor={id}>Download speed (Mbps)</Label>
  <FieldHint id={hintId}>Used to recommend a stream profile…</FieldHint>
  <Input id={id} aria-describedby={describedBy(hintId, error && errorId)} />
  {error && <FieldError id={errorId}>{error}</FieldError>}
  ```

  - `FieldHint` and `FieldError` (`ui/field.tsx`) are the only help and error
    text of a field: both `text-sm`, muted and destructive. `FieldError` is a
    `role="alert"`. No hand-set `<p className="text-xs …">` beside a control.
  - Static help goes between the label and the control, so it is read before
    the control is used and is not covered by a select's list or a phone
    keyboard. What depends on the value goes below: the error, the selected
    option's description, the notice of what is in force.
  - No section title above a field that repeats its label. Settings →
    Playback used to stack "Preferred profile", a description, then the label
    "Profile"; the title is gone and its description is the hint
    (`settings/PlaybackField.tsx`). A heading stays where it groups several
    fields (a provider in External services, a library in Libraries) or
    names a read-only value (`PlaybackSection`).
  - **The visible label is the accessible name.** A control with a
    `<Label htmlFor>` takes no `aria-label`: one that differs ("Your display
    name" over a visible "Name") breaks voice control and WCAG 2.5.3, and one
    that matches is noise that drifts. A button whose name needs its context
    shows it ("Scan movies library", not "Scan Library" with a hidden longer
    name). The exception is an icon-only button, and a row action that
    starts with its visible word ("Rename Living Room TV").
  - A placeholder shows an example of the format (`user@example.com`,
    `/srv/media/movies`), never the label again and never the rule: the
    password rule is the hint "At least 9 characters." (`PASSWORD_HINT`,
    `lib/form-validation.ts`), and the upper limit is explained only by the
    error raised when it is hit.
- **A confirmation with a field is a form.** The "Type DELETE to confirm"
  dialogs submit on Enter once the word is typed (the submit button is
  disabled until then, which also blocks Enter), and their Cancel is
  `type="button"`.
- **What is said about a control is tied to it.** A message that lives away
  from its field is still in that field's `aria-describedby`: General's
  "Static directory is required." sits in the Save bar's status line
  (`statusId`), and the hardware acceleration select is described by its
  help, by what the chosen mode does and by what the server has in force. A
  server refusal goes to the field it is about (a refused new password to
  New password, a 401 to Current password), and a failure that is about no
  field marks none: the toast alone reports it.
- **Settings copy is sentence case**: card titles, sub-headings, labels,
  buttons and dialog titles ("Application behavior", "Current password",
  "Save settings", "Add user"). Names keep their capitals (Quick Connect,
  Apple Music, TMDB, Spotify, PIN). A description is a sentence and ends with
  a period.
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
- **Library stat tiles stay a `grid-cols-3` row at every width.** The Music
  and TV Shows cards on Settings → Libraries render their counts as `StatItem`
  grids ("Music library statistics", "TV shows library statistics"); below
  `sm` a tile drops its icon box and tightens to `gap-2 p-3` with the figure at
  `text-lg` and its label at `text-xs`, which is what three tiles need inside
  the ~278 px a 360 px phone leaves in the card, and from `sm` the icon,
  `gap-4 p-4` and `text-2xl` return. TV Shows counts Shows, Seasons and
  Episodes from `GET /api/shows/stats`: library totals, distinct from the scan
  panel's per-run "local episodes" figure.
- **Card section titles are real headings**: render `CardTitle` with `asChild`
  wrapping an `<h2>` (login's is the page `<h1>`) so card-sectioned pages are
  navigable by heading.
- The header search, the sign-in form and the account and users forms use
  the `glass` field variant: the frosted light treatment from
  `lib/input-styles.ts` — the sanctioned raw-color exception (§1.2); its
  contract is pinned by `shared/input-styles.test.ts`.
- A multi-line field is a `Textarea`, never a hand-styled `<textarea>`; it
  takes the same variants as `Input` (`test/ui/fields.test.tsx`).
- File inputs, checkboxes, selects: use the primitives; selects in
  dialogs rely on the `data-slot` contract (§1.6). Destructive confirmations
  go through the shared `ConfirmDialog`, and paged lists through
  `LibraryPagination` — neither is re-implemented per page. A Save-bar form
  whose save deletes data asks when Save is pressed, not when the control
  changes, since the control only edits the form: turning off "Download
  images" in General settings confirms "Delete downloaded images?" first.
- Form-level failures toast (login's wrong-credentials toast included);
  field-level validation renders inline with
  `aria-invalid` (styled by the Button/Input base classes) and moves focus to
  the first invalid field. A field error is never repeated as a "Validation
  error" toast; `showValidationError` is only for a rule with no field to mark
  (no one invited to a room, a playlist name the dialog shows no slot for).
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
  theirs. Its on/off control is the shared `SwitchField` (the `Switch`
  primitive, a real `role="switch"` button, with its description tied in by
  `aria-describedby`),
  the same row the General settings card uses. Its off track is
  `bg-muted-foreground/25`, a mid grey in both themes, so the white thumb
  reads against it: `bg-accent` is near-white in light and hid the thumb.
- **A service that needs no credentials gets a switch, not an empty
  section.** In General → External Services each provider has a
  `ServiceSection`; one without credentials (Apple Music) holds a
  `SwitchField` instead of secret inputs, saved by the card's Save bar like
  the fields around it. Its description says that no account is needed and
  when the change applies ("applies from the next scan").
- **A setting the server may not honor says what is in force.** Under the
  Server card's hardware acceleration select, a notice read from the saved
  settings (not the form) names the device transcodes actually run on:
  muted helper text when it is the stored device, and `text-destructive`
  with the probe's reason for as long as the startup probe refuses the stored
  one. It also names the transcode cap whenever `max_transcode_height` is
  below the tallest catalog profile.
