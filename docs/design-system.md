# Igloo Design System

This document describes the Igloo visual design system as implemented in the web
client (`web/`), explains the app's UI and UEX, and gives a mapping for recreating
both in a **Kotlin Android TV app built with Jetpack Compose**. It also records
styling issues and improvement suggestions found while documenting it.

The web client is the **source of truth**. This document drives the Android TV
reimplementation — read §1 for the visual language, §2 for the Compose/TV port
strategy, and §3 for the actual screens, navigation, and interaction flows to rebuild.

> Source of truth for the live web tokens: `web/src/assets/styles.css` (OKLCH
> design tokens) and `web/src/assets/boot.css` (pre-React boot styles). Contrast
> is enforced by `web/src/test/contrast.test.ts`.

---

## 1. The design system

### 1.1 Identity & approach

Igloo is an "icy glacier" themed media center. The palette is a cool glacier blue
primary with a sparing warm amber ("aurora") accent, on a cool near-white canvas
(light) or deep navy canvas (dark). **Dark is the default theme.**

The web system is implemented with:

- **Tailwind CSS v4** (`@import "tailwindcss"` + `@theme inline`) — no
  `tailwind.config.js`; configuration lives in CSS.
- **shadcn/ui** ("new-york" style) primitives in `web/src/components/ui`, built on
  `radix-ui` and styled with `class-variance-authority` (CVA).
- **CSS custom properties** as semantic design tokens, defined once per theme
  (`:root` = light, `.dark` = dark) and consumed through Tailwind utility classes
  like `bg-card`, `text-muted-foreground`, `ring-ring`.
- `tw-animate-css` for enter/exit animations, with a strict `motion-reduce:`
  discipline.

Theme switching toggles the `dark` class on `<html>` (`web/src/lib/theme.ts`),
persisted to `localStorage` under `igloo-theme`, with an anti-flash inline script in
`index.html` that applies the stored theme before the first paint. Default is dark
whenever no stored value is present.

For an Android TV port, the takeaway: **one palette, two themes, dark by default, a
single glacier focus color, and a hard motion-reduce rule.** These map directly onto
a Compose theme (§2).

### 1.2 Color tokens (semantic)

Colors are authored in **OKLCH** in `styles.css`; the hex equivalents below come from
the inline comments in that file and from `boot.css`. Tokens are paired (`X` surface +
`X-foreground` text) so foreground/background contrast is guaranteed. All hexes are
verified against `styles.css` as of this writing.

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
| `destructive-foreground` | `#08131F` | `#FFFFFF` | Text on destructive |
| `aurora` | `#F59E0B` | `#F59E0B` | Warm accent (sparing) |
| `aurora-foreground` | `#08131F` | `#08131F` | Text on aurora |
| `success` | `#34D399` | `#059669` | Success state |
| `success-foreground` | `#08131F` | `#08131F` | Text on success |
| `accent-teal` | `#2DD4BF` | `#0D9488` | Secondary accent |
| `accent-teal-foreground` | `#08131F` | `#08131F` | Text on accent-teal |
| `sidebar` | `#0F1A2E` | `#E8F1FA` | Sidebar chrome |
| `sidebar-primary` | `#38BDF8` | `#0369A1` | Active sidebar item |
| `chart-1..5` | glacier / teal / aurora / success / danger | same families | Data viz |

Notes:
- `aurora` and `aurora-foreground` are **identical in both themes**.
- `ring` is deliberately a single focus color across the whole app (glacier blue).
- Many usages apply alpha at the call site via Tailwind's `/NN` modifier
  (`bg-primary/90`, `ring-ring/50`, `bg-black/30`). In Compose these become
  `Color.copy(alpha = …)` — see §2.
- **Untokenized exceptions to be aware of** (do not hunt for a token for these):
  - `boot.css` splash-message greys `#64748B` (light) / `#94A3B8` (dark) are
    hardcoded, not drawn from `muted-foreground`.
  - The media cards use raw black/alpha + white over posters: `MovieCard` uses
    `bg-black/30` (dim overlay), `text-white`, and a `from-black/90` poster gradient;
    `AlbumCard` uses `bg-black/40` (dim overlay). Both share `shadow-black/30`.
    White-on-darkened-poster is a legitimate over-media pattern; port it as literal
    black/alpha + white, not tokens.
- All token pairs pass their contrast budget in both themes (verified by
  `contrast.test.ts`); e.g. `success-foreground` on `success` measures ~4.96:1 (light)
  and ~9.73:1 (dark).

### 1.3 Typography

- **Font family**: `Inter`, then a system-UI fallback stack. Defined **only** in
  `boot.css` on `body` (`Inter, ui-sans-serif, system-ui, -apple-system,
  BlinkMacSystemFont, "Segoe UI", sans-serif`), with `-webkit-font-smoothing:
  antialiased`. It is **not** a CSS custom property / token.
- **Scale**: Tailwind's default type scale, used as utility literals (there is no
  custom font-size token in CSS). Actual usage: `text-sm` and `text-xs` dominate
  body/secondary text; `text-lg`/`text-xl`/`text-2xl` for section and card titles;
  `text-3xl`–`text-5xl` for hero/page headings.
- **Weights**: `font-medium` (controls/labels), `font-semibold` (titles).
- Headings frequently use tight tracking (`tracking-tight` / `-0.02em`) and
  `line-clamp-{n}` for truncation. Over-media text uses `drop-shadow-lg` +
  `text-white` for legibility on posters.

> For a 10-foot TV UI, bump the base type up: TV viewing distance means body copy
> should be materially larger than the web's `text-sm`/`text-xs`. Keep the *hierarchy*
> (medium controls, semibold titles, tight heading tracking); scale the absolute sizes.

### 1.4 Spacing, radius, elevation

- **Radius**: base `--radius: 0.625rem` (10px) — the only non-color scale that is an
  actual CSS token. Aliases: `sm = radius−4px` (6px), `md = radius−2px` (8px),
  `lg = radius` (10px), `xl = radius+4px` (14px). Cards use `rounded-xl`;
  buttons/inputs `rounded-md`; pills `rounded-full`.
- **Spacing**: standard Tailwind 4px scale used as utility literals (`gap-2`, `px-4`,
  `py-6`, etc.) — **not** tokenized. Cards default to `py-6` with `px-6` sections and
  `gap-6` between blocks.
- **Elevation**: `shadow-xs/sm/md/lg/xl/2xl`. Interactive media cards add a colored
  glow on hover (`hover:shadow-primary/20`) — this becomes a **focus** glow on TV.

### 1.5 Motion

Centralized in `web/src/lib/constants.ts` as `MOTION_*` / `CARD_*` class constants
and duration tokens:

- Durations: `MOTION_DURATION_MICRO_MS = 150`, `MOTION_DURATION_STANDARD_MS = 200`,
  `MOTION_DURATION_PAGE_MS = 300`.
- Transitions are property-scoped (e.g.
  `transition-[background-color,border-color,color,box-shadow,opacity]`) rather than
  `transition-all`.
- **Every** animation includes a `motion-reduce:` fallback that disables or
  neutralizes it. This is a hard accessibility rule (see §1.7).
- Reusable patterns: `CARD_SURFACE_CLASS` (hover lift + glacier glow),
  `CARD_MEDIA_HOVER_CLASS` (poster zoom), `MOTION_PAGE_ENTER_CLASS`,
  `MOTION_MEDIA_OVERLAY_ENTER_CLASS`, `MOTION_PLAYER_CHROME_ENTER_CLASS`, etc.

### 1.6 Component variants (the contract)

The `Button` (`web/src/components/ui/button.tsx`) is the clearest expression of the
system. Variants: `default`, `destructive`, `outline`, `secondary`, `ghost`, `link`,
`accent`, `accent-pill`, `aurora`. Sizes: `default`, `xs`, `sm`, `lg`, plus icon sizes
`icon`, `icon-xs`, `icon-sm`, `icon-lg`. All share a base with focus-ring
(`focus-visible:ring-ring/50 ring-[3px]`), disabled opacity, and `aria-invalid`
styling.

Other tokenized primitives: `Card`, `Dialog`/`Sheet`/`AlertDialog`, `Select`,
`DropdownMenu`, `Tabs`, `Tooltip`, `Popover`, `Alert`, `Input`, `Checkbox`, `Avatar`,
`Pagination`, `Skeleton`, `Spinner`, `Sidebar`. Tabs everywhere share one look:
`bg-muted/50` pill container, active trigger `bg-primary text-primary-foreground
shadow-primary/20`. Shared cross-component class strings (select trigger styling, card
surfaces) are exported as constants from `constants.ts`.

### 1.7 Accessibility (non-negotiable)

- A single, consistent focus ring (`ring`) with `focus-visible:ring-[3px]` /
  `ring-2 ring-offset-2`.
- `motion-reduce:` on every transition/animation.
- Contrast budget enforced in CI by `contrast.test.ts`: body text ≥ 7:1 (AAA), all
  other foreground/surface pairs ≥ 4.5:1 (AA) — in **both** themes.
- Live regions (`LiveAnnouncer`), skip links, ARIA semantics are preserved throughout
  (see `CLAUDE.md`).

> On Android TV, accessibility centers on **TalkBack** and **clear focus visibility**
> rather than pointer focus rings. Reuse the `ring` glacier color as the focus
> highlight, keep the motion-reduce rule (§2.3), and preserve content descriptions on
> every focusable/actionable element.

---

## 2. Rebuilding on Android TV with Jetpack Compose

Compose has no Tailwind, no CSS variables, no OKLCH, and no `@media`-driven theming.
The strategy is to **port the semantic tokens to an immutable Kotlin theme object,
expose the active theme via a `CompositionLocal`, and consume tokens through it** —
mirroring the web system 1:1 so the two clients stay conceptually aligned. Because this
is a **TV** target, the single biggest translation is **hover → d-pad focus** (§2.2).

Toolkit: build on **`androidx.tv.material3`** (the TV variant of Material 3, with
`Surface`, `Card`, `Button`, `Tab`, and TV-aware focus/indication) plus
`Modifier.focusable()`, `Modifier.onFocusChanged`, and focus restoration
(`FocusRequester` / `focusRestorer`). Use `androidx.tv.foundation` lists
(`TvLazyColumn` / `TvLazyRow`) for the horizontal/vertical rails.

### 2.1 Tokens as a Compose theme

- Represent each theme as an **immutable data object** of `Color` tokens — one for
  light, one for dark — mirroring the table in §1.2. Compose `Color(0xFFxxxxxx)` takes
  ARGB hex; the OKLCH → hex conversion is already done (use the hexes in §1.2
  directly). No runtime OKLCH is needed.
- Expose the active theme via a `CompositionLocal` (e.g. `LocalIglooColors`), the
  Compose analog of the `.dark` class. Select the palette from a mode state that
  **defaults to dark** and is persisted with **DataStore** under the key `igloo-theme`
  (same key/semantics as the web `localStorage`). Hydrate on launch before the first
  frame to avoid a flash.
- Non-color scales are theme-independent constants: `radius` (sm 6 / md 8 / lg 10 /
  xl 14 dp), the Tailwind 4px `spacing` steps, a font-size scale (scaled up for TV per
  §1.3), and `duration` (micro 150 / standard 200 / page 300 ms).
- **Alpha modifiers**: the web applies alpha at the call site (`bg-primary/90`,
  `ring-ring/50`, `bg-black/30`). In Compose use `token.copy(alpha = 0.90f)` etc., or
  precompute the common ones.

### 2.2 The 10-foot / d-pad focus model (the heart of a TV port)

The web UI leans heavily on **pointer hover**, which does not exist on a TV remote.
Every hover affordance must become a **focus** affordance. The good news: the web
already pairs hover with `focus-within` on the cards, so the *intent* is consistently
"reveal/emphasize the active item" — on TV that trigger is simply d-pad focus.

| Web pattern | Android TV / Compose equivalent |
|-------------|---------------------------------|
| `group-hover` / `group-focus-within` overlay + center Play button reveal | Track focus with `Modifier.onFocusChanged`; show the dim overlay + Play affordance when the card (or its focus group) is focused |
| `group-hover:scale-105` poster zoom | `animateFloatAsState` scale ~`1.05f` driven by focus state |
| `hover:-translate-y-1` + `hover:shadow-primary/20` lift + glacier glow | Focused elevation/scale + a glow/border using the `ring` glacier color (`androidx.tv.material3` `Surface` scale/glow/border indication) |
| `focus-visible:ring-[3px]` (the single `ring` color) | The TV focus highlight — one consistent glacier border/glow reused everywhere |
| `TrackItem` Play button hidden until hover (musician/album variants) | Always show it, or reveal on **row focus** — never gate an action behind hover |
| Idle-hide player chrome on pointer move (`useIdleControls`) | Show chrome on any d-pad/media-key event; auto-hide after an idle timeout |

Also design **focus restoration** (returning to the last-focused card when coming back
to a rail/grid) and **spatial d-pad traversal** across the sidebar ↔ content boundary.

### 2.3 Web → Compose mapping table

| Web (Tailwind / CSS) | Jetpack Compose equivalent |
|----------------------|----------------------------|
| `bg-card`, `text-card-foreground` | `colors.card`, `colors.cardForeground` from the `CompositionLocal` |
| `text-foreground` | `colors.foreground` |
| `.dark` class toggle | mode state → `CompositionLocal` palette (default dark) |
| `bg-primary/90` (alpha modifier) | `colors.primary.copy(alpha = 0.90f)` |
| `rounded-xl` | `RoundedCornerShape(radius.xl)` (14.dp) |
| `shadow-lg` / `hover:shadow-primary/20` | `Surface` tonal/shadow elevation; focus glow via TV `Surface` glow/border |
| `gap-6`, `px-6`, `py-6` | `Arrangement.spacedBy(24.dp)`, `Modifier.padding(...)` |
| `transition-* duration-200` | `animate*AsState(tween(durationMillis = 200))` |
| `motion-reduce:` | check `Settings.Global.ANIMATOR_DURATION_SCALE == 0` (or accessibility reduce-motion) → skip/snap the animation |
| `focus-visible:ring-*` | focus-driven indication reusing the `ring` color (§2.2) |
| `bg-linear-to-t from-black/90` gradient | `Brush.verticalGradient(...)` as a `Box` overlay |
| `drop-shadow-lg` on text | `TextStyle(shadow = Shadow(...))` |
| `line-clamp-2` | `Text(maxLines = 2, overflow = TextOverflow.Ellipsis)` |
| `aspect-2/3` | `Modifier.aspectRatio(2f / 3f)` |
| `backdrop-blur` | `Modifier.blur(...)` / a blurred image layer (or a solid scrim) |
| `group-hover` reveals | focus state, not touch/hover (§2.2) |
| remote images (posters/backdrops) | Coil (`AsyncImage`) |

### 2.4 Things that do not translate (and what to do)

- **Hover & `group-hover`**: no hover on a remote. Drive every reveal/emphasis from
  **focus** (§2.2). The poster "reveal on hover" overlays become "reveal on focus."
- **Pointer focus rings (`ring`)**: replace with the platform focus engine's visible
  focused-item style, reusing the `ring` glacier color for a consistent highlight.
- **OKLCH & alpha modifiers**: pre-convert all tokens to hex (done in §1.2); apply
  alpha with `Color.copy(alpha = …)`.
- **`@media (prefers-reduced-motion)`**: replace with the Android animator-duration /
  reduce-motion check before running any animation — this preserves the web's hard
  `motion-reduce` rule.
- **Tailwind utility-merging (`cn`/`tailwind-merge`)**: replace with `Modifier` chains;
  later modifiers refine earlier ones, which is the Compose analog of "last class wins."
- **Text entry** (search, login): a full keyboard is painful on a remote. Prefer the
  TV leanback/on-screen keyboard and keep text entry to the few places that need it
  (login, search).

---

## 3. App UI & UEX

What to build. This section describes the actual product surfaces so a Compose team
knows the screens, navigation, and flows. Web file paths are cited for reference — read
them for exact layout details.

### 3.1 Navigation spine & shell

- **Auth boundary**: all real content lives under the pathless `_auth` route
  (`web/src/routes/_auth/route.tsx`), whose `beforeLoad` redirects unauthenticated
  users to `/login`. `/login` (`web/src/routes/login.tsx`) redirects back in if already
  authed. So the top-level split is **login (no chrome)** vs **authenticated app (full
  shell)**.
- **Shell** (`web/src/components/AppShell.tsx`): a persistent **left sidebar** + a right
  content pane with a **sticky top header** and a single scrolling content column that
  renders the active route.
- **Sidebar** (`web/src/components/app-sidebar.tsx`) — the primary navigation. Six
  destinations, each icon + label: **Home** (`/`), **Movies** (`/movies`), **TV Shows**
  (`/tv-shows`), **Music** (`/music`), **Photos** (`/photos`), **Settings**
  (`/settings`). Header shows an "I" logo tile + "Igloo" wordmark; footer has **Logout**.
  Active item: `bg-sidebar-accent` + primary-colored icon; inactive: muted.
- **Header** (`web/src/components/Header.tsx`): a **search** form (submits to
  `/search?q=…`), a **notification bell**, and the **theme toggle** (light/dark).

> **On TV**: the sidebar is the primary **vertical d-pad nav spine** (six
> destinations). The mobile-overlay/hamburger behavior is irrelevant; treat it as an
> always-present left rail. D-pad **right** from the rail enters the content grid;
> **left** from the first content column returns focus to the rail.

### 3.2 Key screens

- **Home** (`_auth/index.tsx`) — dashboard: a welcome hero panel, then stacked
  **horizontal card rows** (`WatchRooms`, `LatestMovies`, `LatestAlbums`,
  `MoviesInTheaters`). This row-of-rails layout is the most TV-native screen — a strong
  model for the TV home.
- **Movies index** (`_auth/movies/index.tsx`) — header + stats + a "More" menu, then a
  **3-tab** control (All Movies / Genres / Playlists; tab state in the URL). All Movies
  is a **poster grid** (2→6 columns responsive) with A–Z/Z–A sort and pagination.
- **Movie detail** (`_auth/movies/$id/index.tsx`) — the richest screen: a **full-bleed
  backdrop** with a bottom `from-background` gradient scrim, content pulled up over it,
  **two-column** on wide (poster left; title/tagline/**metadata chips**/genres/**hero
  actions** right). Hero actions (`MovieDetailsHeroActions.tsx`): **Play** (→
  `/movies/$id/play` with resolved mode/audio/subtitle params), **Watch/Watched**
  toggle, **Like**, and a **More** menu (Playback Settings, Watch Together, Edit
  [admin], Technical Details, Delete [admin]). Below: `CastSection`,
  `MovieChaptersSection`, extra details, YouTube videos, production companies.
- **Music** (`_auth/music/index.tsx`) — header + stats + a "More" menu, then **4 tabs**:
  **Musicians** (grid of circular cards), **Albums** (grid of square cards), **Tracks**
  (a **window-virtualized** flat list with A–Z letter headers + Play all / Shuffle all),
  **Playlists**. Album detail (`_auth/music/album.$id.tsx`) and Musician detail
  (`_auth/music/musician.$id.tsx`) follow the backdrop + hero + list pattern.
- **Settings** (`_auth/settings/route.tsx`) — a **tab bar** (General / Account /
  Libraries / Playback / Users) whose tabs are child routes; **Users** is admin-only.
- **Search results** (`_auth/search/index.tsx` loader + `index.lazy.tsx` UI) — reached
  from the header search form (`?q=…`). A heading ("Search results for '{query}'") over a
  **5-tab** control (**All / Movies / Albums / Musicians / Tracks**; `tab` + `page` in the
  URL). The **All** tab stacks up to four sections (movies/albums/musicians/tracks), each
  with a count and a **"See all →"** link into that category's own tab; the per-category
  tabs are paginated grids/lists reusing `MovieCard` / `AlbumCard` / `MusicianCard` /
  `TrackItem`. `SEARCH_PER_PAGE = 24`, with numbered `LibraryPagination` (not infinite
  scroll). Before any query it shows a prompt; no matches shows a centered empty state.
- **Login** (`login.lazy.tsx`) — full-bleed background image + dark overlay, a centered
  card (logo, email + password with show/hide, accent "Sign in").
- **TV Shows** and **Photos** are `ComingSoon` placeholders today.

### 3.3 Media card anatomy

All media cards share style constants in `web/src/lib/constants.ts`
(`CARD_SURFACE_CLASS`, `CARD_MEDIA_HOVER_CLASS`, `CARD_OVERLAY_REVEAL_CLASS`,
`CARD_ACTION_REVEAL_CLASS`): a rounded-xl bordered `bg-card` surface, poster zoom
(`group-hover:scale-105`), and a lift + glacier glow on hover — all of which become
**focus** treatments on TV (§2.2).

- **`MovieCard.tsx`** — **2:3 poster** (`aspect-2/3`), `object-cover`. A permanent
  bottom `from-black/90` gradient carries a white **title** (`line-clamp-2`) + year over
  the poster. On focus (web: hover/focus-within): a `bg-black/30` dim overlay + a
  centered circular **Play** button linking straight to `/movies/$id/play`.
- **`AlbumCard.tsx`** — **square** cover (`aspect-square`), title + artist below. On
  focus: a `bg-black/40` overlay + a Play button that **plays the album in the global
  audio player** (not navigation). The card itself links to the album detail.
- **`MusicianCard.tsx`** — **circular** thumbnail, centered name + "N albums · M
  tracks". No Play button; focus just zooms + lifts.
- **`TrackItem.tsx`** — list row, variants `library | playlist | musician | album`. In
  album variant a track index swaps to a spinner/primary color when playing. **Caveat**:
  in musician/album variants the Play button is `sm:opacity-0` until hover (`sm:opacity-0
  sm:group-hover:opacity-100`), so it hides only on `sm`+ screens and is always visible on
  touch/small screens — on TV, always show it or reveal on **row focus**. Rows also carry a
  Like heart and an actions menu.

### 3.4 Media playback UX

- **Movie player** (`_auth/movies/$id/play.tsx` + `web/src/components/VideoPlayer.tsx`
  + `MoviePlayerControls.tsx`): the play route resolves playback **mode** (`direct` vs
  **HLS transcode**), audio track, and subtitle track from user prefs + stream
  capabilities, then plays via a `<video>` element (HLS.js or native HLS) with
  start-time seeking, subtitle injection, and HLS session-lost recovery. Layout: a top
  **header bar** (title + Back) and a bottom **controls footer** that **auto-hide after
  idle** (`useIdleControls`) and reappear on input. Controls: a **progress/seek bar**,
  current/total time, Rewind / big Play-Pause / FastForward, a **quality-label chip**, a
  **chapter menu**, a **volume control**, and a **fullscreen** toggle. A **Resume
  dialog** offers "Resume" vs "Start from beginning"; progress is persisted and Media
  Session metadata is set.
- **Keyboard shortcuts** (`useVideoPlaybackKeyboard.ts`) — the natural **d-pad /
  media-key mapping for TV**: Space/K play-pause, J/← rewind, L/→ forward, ↑/↓ volume, M
  mute, F fullscreen, Esc exit, Home/0 restart.
- **Global audio player** (`web/src/components/AudioPlayer.tsx`, provided app-wide via
  `AudioPlayerContext`): a **persistent** player — a minimized docked bar (cover,
  title/artist, prev/play/next, progress, volume) that expands to fullscreen. Album/track
  Play buttons across the app feed it. It pauses and yields its keys on the video pages.
- **Watch rooms** (`web/src/components/watch-room/WatchRoomPage.tsx`): **synchronized
  group watch** — a player panel + a members panel (host badge, connected/away status),
  with play/pause/seek broadcast over a realtime connection so all members stay in sync.
  Host can close the room; others leave.

> On Android use **Media3 / ExoPlayer** for playback (it handles HLS + transcode
> streams and the Media Session), and map the keyboard shortcuts above onto d-pad +
> the remote's media keys.

### 3.5 UI states — loading, empty, error (cross-cutting)

There is **no single shared `EmptyState`/`ErrorState` component**; each screen inlines its
states, but they follow a small set of repeated recipes — worth building **once** as shared
composables on TV.

- **Loading — two tiers.** Route transitions use one app-wide pending component
  (`RouterPending` → `AppLoadingScreen`: full-screen dim + a centered card with a pulsing
  glacier `Snowflake`, `role="status"`; wired via `defaultPendingComponent` in `App.tsx`).
  Within a screen, each query does `if (isLoading) return <XxxSkeleton/>`, and skeletons
  are **grid-matched to the real layout to avoid layout shift** — e.g. the movies grid
  skeleton renders `MOVIES_PER_PAGE` placeholder cards in the identical
  `grid-cols-2 … lg:grid-cols-6`. The placeholder-card recipe (bordered `rounded-xl bg-card`,
  an `aspect-2/3 bg-muted` poster, two `bg-muted` text bars, `animate-pulse`) recurs across
  movies/search/music. Primitives: `Skeleton` (`ui/skeleton.tsx`), `Spinner`
  (`ui/spinner.tsx`, `role="status"`), plus bespoke skeletons (`MovieDetailsSkeleton.tsx`).
  Shimmer/spin classes (`MOTION_LOADING_STATE_CLASS`, `MOTION_SPINNER_STATE_CLASS` in
  `constants.ts`) both carry `motion-reduce:animate-none`.
- **Empty — two variants.** (A) *Minimal*: `py-12 text-center text-muted-foreground` with a
  large faded lucide icon (`size-10 opacity-50`) + one line ("No movies found in your
  library."). (B) *Rich CTA* (empty playlists): a gradient icon orb
  (`bg-linear-to-br from-muted … to-primary/30`), an `<h3>`, a description, and a primary
  pill button. Empty states also emit a `LiveAnnouncer` message.
- **Error — inline card + Retry.** Detection is uniform: `isError || isApiFailure(data)`
  (the API returns an `{ error, data }` envelope; `is-api-failure.ts` catches `error===true`).
  Shared cards: `MoviesLoadError.tsx` (`role="alert"`, `border-destructive/25
  bg-destructive/10 text-destructive`, a "Try again" link that calls `refetch()`); detail
  pages use `MediaNotFound.tsx` (shadcn `Alert variant="destructive"`). **Mutation/action**
  errors don't render inline — they toast via Sonner (`toast-helpers.ts`
  `showActionFailed(...)`; top-right, `richColors`, `border-destructive/50` on `bg-card`).
- **Tab panels cross-fade** on change (`CONTENT_FADE_ENTER/EXIT` via
  `useContentFadeTransition.ts`).

> **On TV:** build these **once** as shared composables — an `IglooLoading` (grid-matched
> shimmer), an `IglooEmpty` (icon + text, plus the rich-orb CTA variant), and an
> `IglooError` (retry button that re-runs the query) — since the web only lacks them by
> accident. Keep skeletons grid-matched so **focus position doesn't jump** when content
> arrives. Preserve `motion-reduce` → the Android reduce-motion check (§2.3). Route-level
> loading → one app pending screen mirroring `AppLoadingScreen`. Announce state changes to
> **TalkBack** the way `LiveAnnouncer` does on web.

### 3.6 Notifications

- **`NotificationBell.tsx`** (in the header, beside the theme toggle) is a **popover** — not
  a page or sheet. Trigger: a ghost bell icon button with a glacier **unread badge**
  (`bg-primary text-primary-foreground` pill, "99+" past 99) shown only when
  `unreadCount > 0`; the count is also in the button's `aria-label`.
- **Panel** (`w-80 bg-card`): a header row ("Notifications" + a **"Mark all read"** link when
  anything is unread), then a `max-h-96` scroll body with states — a non-blocking "Unable to
  refresh" banner (keeps the stale list), an initial `Spinner`, an empty state ("You're all
  caught up."), or a `divide-y` list.
- **Row**: unread rows are tinted `bg-muted/40` and carry a glacier **unread dot**; each
  shows a **type label** (`movie_request → "Movie request"`, etc.), the message, an optional
  "From {name}", a right-aligned **relative time**, and a per-row **dismiss (X)** button.
  Clicking an unread row marks it read.
- **Data**: the unread-count query **polls every 30 s** (always mounted); the full list query
  is **`enabled` only while the popover is open** (`staleTime: 0`). Mark-read / mark-all /
  delete mutations invalidate both query keys; failures toast via `showActionFailed`. Backend
  contract is the `/api/notifications*` routes (see `docs/openapi.json`).

> **On TV:** a ~320px popover anchored to a header bell can work, but the rows and the X
> button here **rely on default focus styling** — add explicit d-pad focus treatment (reuse
> the `ring` glacier highlight, §2.2) to every row-button and dismiss button, and make sure
> a focused unread row's "mark read" and its dismiss action are both reachable (two focus
> targets per row). Keep the 30 s badge poll; gate the list fetch on the panel being open. A
> full side panel may read better than a small popover at 10 feet.

---

## 4. Web-side styling issues found during exploration

Concrete, fixable inconsistencies in the current web code. Status is current as of this
revision.

1. **Missing token source of truth — STILL OPEN.** `styles.css` (line ~102) cites
   `docs/igloo-theme.ts` as the "hex source of truth," but **that file does not exist**
   (nor does `web/src/lib/tokens.ts`) — a **dangling reference**. The hexes only live in
   CSS comments, so there is no machine-readable token source.

2. **Theme constants duplicated in four places — STILL OPEN.** Kept in sync by hand: the
   OKLCH tokens (+ hex comments) in `styles.css`, the hexes in `boot.css`, the inline
   anti-flash script in `index.html`, and `THEME_COLORS`/`THEME_TEXT_COLORS` in
   `src/lib/theme.ts`. Currently consistent, but a real drift hazard (the code comments
   acknowledge it).

3. **Sonner toaster colors — FIXED.** The toaster (`web/src/components/ui/sonner.tsx`)
   now uses semantic tokens (`!bg-card`, `!border-success/50`, `!border-destructive/50`,
   `!text-card-foreground`) instead of the old raw `bg-emerald-900/90` / `bg-red-900/90`
   / `text-emerald-100`.

4. **Hardcoded card colors — STILL OPEN.** `MovieCard.tsx` and `AlbumCard.tsx` use raw
   `bg-black/30`, `bg-black/40`, `text-white`, `from-black/90`, `shadow-black/30`. The
   white-over-darkened-poster cases are legitimate, but they won't track the palette and
   have no token equivalent. `boot.css` splash greys (`#64748B` / `#94A3B8`) are
   similarly untokenized.

5. **Toaster contrast not covered by `contrast.test.ts` — STILL OPEN.** The contrast
   test guards **token pairs** only; it does not test the toaster's actual rendered
   combination (`text-card-foreground` on `bg-card` with a translucent success/
   destructive border), so that could regress silently.

6. **Stale "dark-boot / toggle-not-shipped" comment — FIXED.** No such comment remains;
   the light/dark toggle shipped (`ThemeToggle.tsx`, used in `Header.tsx`).

7. **No exported numeric scale for spacing/typography — STILL OPEN.** Radius is a token
   (`--radius` + aliases), but spacing and type rely entirely on Tailwind utility
   literals scattered across components — there is no single place defining the intended
   scale (this matters for keeping web and the Android client aligned).

---

## 5. Suggestions for improvements

1. **Create a single token source of truth.** Add `web/src/lib/tokens.ts` (or the
   `docs/igloo-theme.ts` the comments already promise): one typed object with hex +
   OKLCH per token. Generate the `styles.css` `:root`/`.dark` blocks, `boot.css`, the
   `index.html` anti-flash script, and `theme.ts` `THEME_COLORS` from it (or have them
   import shared constants), eliminating the four-way manual sync — and fix the dangling
   `styles.css:102` reference.

2. **Tokenize the remaining raw colors.** Where feasible, replace card `bg-black/*` /
   `text-white` and the boot splash greys with tokens (add tokens if needed) so they
   follow the theme and port cleanly. Keep genuinely over-media white/black where it is
   the correct choice, but document it.

3. **Extend `contrast.test.ts`** to cover the toaster's rendered success/error
   combination and any other hardcoded UI colors, keeping the AA/AAA guarantee
   comprehensive.

4. **Publish the non-color scales as tokens** (spacing, font-size, font-weight,
   duration) in the same shared module, so the web `@theme`, the boot styles, and the
   Android theme all consume identical numbers.

5. **Own the TV focus model as part of the shared system.** Since the web leans on hover
   reveals + pointer focus rings and Android TV is a stated target, treat the
   focus-driven equivalents (reuse the `ring` token for the focused-item highlight,
   convert every hover reveal to a focus reveal — see §2.2) as a first-class part of the
   design system rather than a per-client afterthought. Design focus restoration and
   sidebar ↔ content d-pad traversal up front.
