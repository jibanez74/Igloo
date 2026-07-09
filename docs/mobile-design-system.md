# Igloo Mobile Design System

This document describes how to recreate the Igloo visual design system and core
app experience in a **React Native mobile application for iOS and Android** using
React Native `StyleSheet`. It is intentionally standalone so a mobile engineer can
work from this file without cross-reading the Android TV design-system document.

The web client remains the **source of truth** for the current visual language.
This document translates that system into mobile conventions: touch-first
navigation, phone and tablet layouts, native safe areas, VoiceOver/TalkBack
accessibility, and React Native style objects instead of Tailwind classes or CSS
variables.

> Source of truth for live web tokens: `web/src/assets/styles.css` (OKLCH design
> tokens), `web/src/assets/boot.css` (pre-React boot styles), and
> `web/src/lib/theme.ts` (theme persistence keys and meta colors). Contrast is
> enforced by `web/src/test/contrast.test.ts`.

---

## 1. The Design System

### 1.1 Identity & approach

Igloo is an "icy glacier" themed media center. The palette uses a cool glacier
blue primary, a sparing warm amber ("aurora") accent, and a cool near-white
canvas in light mode or deep navy canvas in dark mode. **Dark is the default
theme.**

The web system is implemented with Tailwind CSS v4, shadcn/ui primitives, and
CSS custom properties. A React Native mobile app should preserve the same
semantic tokens and component intent, but implement them with:

- **React Native `StyleSheet.create`** for named, typed style objects.
- **Style arrays** for composition and state overrides, where later entries take
  precedence.
- **A typed theme object** exposed through React context instead of CSS custom
  properties.
- **`Pressable` state styles** for pressed, focused, and disabled states.
- **`useWindowDimensions`** for responsive layout decisions.
- **`react-native-safe-area-context`** for safe areas; do not use React Native's
  deprecated built-in `SafeAreaView`.

Do not port Tailwind, NativeWind, shadcn/ui, Radix, or CSS variables into the
mobile app unless explicitly chosen later. The mobile design system should feel
native while retaining Igloo's color, density, card anatomy, and media-first
hierarchy.

### 1.2 Color tokens

Colors are authored in OKLCH on the web. React Native should consume the hex
equivalents below as semantic tokens. Token pairs are intentional: surfaces and
their matching foreground colors are chosen to preserve contrast.

| Token | Dark (default) | Light | Role |
|-------|----------------|-------|------|
| `background` | `#0A1322` | `#F2F7FC` | App canvas |
| `foreground` | `#F8FAFC` | `#0A1322` | Primary text |
| `card` | `#15233A` | `#FFFFFF` | Raised surface |
| `cardForeground` | `#F8FAFC` | `#0A1322` | Text on cards |
| `popover` | `#1B2B45` | `#FFFFFF` | Overlay, sheet, menu surface |
| `popoverForeground` | `#F8FAFC` | `#0A1322` | Text on overlays |
| `primary` | `#38BDF8` | `#0369A1` | Primary actions / brand |
| `primaryForeground` | `#08131F` | `#FFFFFF` | Text on primary |
| `secondary` | `#0F1A2E` | `#E3EDF7` | Secondary surface |
| `secondaryForeground` | `#F8FAFC` | `#0A1322` | Text on secondary |
| `muted` | `#0F1A2E` | `#E3EDF7` | Subtle surface |
| `mutedForeground` | `#8094AE` | `#475569` | Secondary text |
| `accent` | `#0F1A2E` | `#E3EDF7` | Low-emphasis selected surface |
| `accentForeground` | `#F8FAFC` | `#0A1322` | Text on accent |
| `border` | `#2A3C57` | `#CBD9E8` | Borders and dividers |
| `input` | `rgba(255,255,255,0.08)` | `#CBD9E8` | Input borders |
| `ring` | `#38BDF8` | `#0EA5E9` | Focus / active outline |
| `destructive` | `#F87171` | `#DC2626` | Danger / delete |
| `destructiveForeground` | `#08131F` | `#FFFFFF` | Text on destructive |
| `aurora` | `#F59E0B` | `#F59E0B` | Warm accent, used sparingly |
| `auroraForeground` | `#08131F` | `#08131F` | Text on aurora |
| `success` | `#34D399` | `#059669` | Success state |
| `successForeground` | `#08131F` | `#08131F` | Text on success |
| `accentTeal` | `#2DD4BF` | `#0D9488` | Secondary accent |
| `accentTealForeground` | `#08131F` | `#08131F` | Text on accent teal |
| `sidebar` | `#0F1A2E` | `#E8F1FA` | Web sidebar / tablet rail chrome |
| `sidebarForeground` | `#F8FAFC` | `#0A1322` | Text on rail chrome |
| `sidebarPrimary` | `#38BDF8` | `#0369A1` | Active rail item |
| `chart1..5` | glacier / teal / aurora / success / danger | same families | Data viz |

Notes:

- `aurora` and `auroraForeground` are identical in both themes.
- `ring` is the single focus color across the product. On mobile it appears in
  focused external-keyboard states, selected tab indicators, active segmented
  controls, and prominent pressed outlines.
- Web alpha modifiers such as `bg-primary/90`, `ring-ring/50`, and
  `bg-black/30` become `withAlpha(colors.primary, 0.9)` or literal
  `rgba(...)` values in React Native.
- Poster and backdrop overlays intentionally use literal black/alpha plus white
  text. Keep that over-media pattern; do not force it into semantic surface
  tokens.

### 1.3 Typography

The web client uses Inter with a system fallback stack. A mobile app should use
the platform system fonts unless a product decision is made to bundle Inter.
System fonts keep text rendering, dynamic type, and fallback behavior predictable
on iOS and Android.

- **Family**: platform default system font. If Inter is bundled later, apply it
  through centralized text components, not ad hoc per-screen styles.
- **Scale**: preserve the web hierarchy, but adapt absolute sizes for mobile.
  Body text should default to 16, secondary text to 13-14, compact metadata to
  12, section titles to 20-24, and hero titles to 28-36 depending on width.
- **Weights**: use `500` for controls and labels, `600` for titles, and avoid
  heavy weights except for short hero/title moments.
- **Line height**: set explicit line heights for reusable text styles so cards
  and rows remain stable across platforms.
- **Truncation**: use `numberOfLines` and `ellipsizeMode`; do not rely on web
  line-clamp behavior.

React Native does not support every CSS text property. Keep letter spacing at
`0` unless a specific native text style proves necessary, and avoid negative
letter spacing.

### 1.4 Spacing, radius, elevation

Use the web's spacing feel without copying Tailwind as a runtime dependency.

| Scale | Value | Mobile use |
|-------|-------|------------|
| `space.1` | 4 | Tight icon/text gaps |
| `space.2` | 8 | Control internals |
| `space.3` | 12 | Compact row gaps |
| `space.4` | 16 | Screen gutters on small phones |
| `space.5` | 20 | Larger row/card padding |
| `space.6` | 24 | Section spacing and tablet gutters |
| `space.8` | 32 | Major screen breaks |

Radius mirrors the web token:

- `radius.sm = 6`
- `radius.md = 8`
- `radius.lg = 10`
- `radius.xl = 14`
- `radius.full = 999`

Elevation should be restrained. Mobile cards should rely primarily on surface,
border, and poster imagery. Use platform shadows only for overlays, sticky
players, sheets, and active card emphasis. On Android, pair shadow intent with
`elevation`; on iOS, define `shadowColor`, `shadowOpacity`, `shadowRadius`, and
`shadowOffset` together.

Use `StyleSheet.hairlineWidth` for crisp separators and subtle card borders when
a full 1-pixel border reads too heavy on high-density screens.

### 1.5 Motion

The web motion constants are:

- `micro`: 150 ms
- `standard`: 200 ms
- `page`: 300 ms

Mobile should keep the same timing vocabulary. Prefer property-specific
animations for opacity, transform, and sheet/player chrome transitions. Avoid
animating layout-heavy properties in long lists.

Every animation must respect reduced motion. Create one shared accessibility hook
around `AccessibilityInfo.isReduceMotionEnabled()` and
`AccessibilityInfo.addEventListener("reduceMotionChanged", ...)`. When reduced
motion is enabled, skip decorative transitions, snap animated values to their end
state, and keep only necessary state changes.

### 1.6 Component contract

The web `Button` variants define the component language. Mobile should implement
the same semantic variants, even if the underlying components are native:

- `default`: primary action, `primary` on `primaryForeground`.
- `destructive`: delete/remove, `destructive` on `destructiveForeground`.
- `outline`: transparent or low-fill control with `border`.
- `secondary`: muted secondary action.
- `ghost`: icon/navigation action with no resting background.
- `link`: inline text action.
- `accent` / `accentPill` / `aurora`: sparing promotional or high-emphasis
  accents.

All interactive components must expose a disabled state, pressed state, accessible
label, and visible selected/focused state where applicable. Icon-only buttons
must always have an `accessibilityLabel`.

---

## 2. React Native Implementation Strategy

### 2.1 Theme objects instead of CSS variables

Define the palette as immutable TypeScript objects, then expose the active theme
through context. Persist the selected mode under the same semantic key as the web
client: `igloo-theme`. Default to dark when the value is missing or invalid.

```ts
type ThemeMode = "light" | "dark";

type IglooColors = {
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
  destructiveForeground: string;
  aurora: string;
  auroraForeground: string;
  success: string;
  successForeground: string;
  accentTeal: string;
  accentTealForeground: string;
};
```

Build shared tokens for `colors`, `spacing`, `radii`, `typography`, `durations`,
and `shadows`. Do not scatter raw hexes through screen styles except for
documented over-media black/white overlays.

### 2.2 StyleSheet patterns

Use `StyleSheet.create` for stable named styles. For theme-dependent styles, use
a small factory that receives tokens and returns a style object. Keep factories
close to shared components or centralize them for primitives; avoid one global
file containing every screen's styles.

```ts
function createButtonStyles(tokens: IglooTheme) {
  return StyleSheet.create({
    base: {
      minHeight: 44,
      borderRadius: tokens.radii.md,
      alignItems: "center",
      justifyContent: "center",
      flexDirection: "row",
      gap: tokens.spacing[2],
      paddingHorizontal: tokens.spacing[4],
    },
    default: {
      backgroundColor: tokens.colors.primary,
    },
    defaultText: {
      color: tokens.colors.primaryForeground,
      fontSize: tokens.typography.control.fontSize,
      lineHeight: tokens.typography.control.lineHeight,
      fontWeight: "500",
    },
    pressed: {
      opacity: 0.86,
      transform: [{ scale: 0.98 }],
    },
    disabled: {
      opacity: 0.5,
    },
  });
}
```

Use style arrays for composition:

```tsx
const [isFocused, setIsFocused] = useState(false);

<Pressable
  accessibilityRole="button"
  accessibilityLabel="Play movie"
  disabled={disabled}
  onFocus={() => setIsFocused(true)}
  onBlur={() => setIsFocused(false)}
  style={({ pressed }) => [
    styles.base,
    styles.default,
    isFocused && styles.focused,
    pressed && styles.pressed,
    disabled && styles.disabled,
  ]}
>
  <Text style={styles.defaultText}>Play</Text>
</Pressable>
```

Guidelines:

- The last style in an array wins; use this instead of Tailwind merge semantics.
- Prefer numeric constants from tokens over inline numbers.
- Use `StyleSheet.absoluteFill` for full-cover poster/backdrop overlays.
- Use `useWindowDimensions` in components that adapt columns, tablet rails, or
  player layouts.
- Track focus with `onFocus` and `onBlur` when a control needs an explicit
  external-keyboard or accessibility focus style.
- Accept a `style` prop on reusable components and apply it last to allow local
  layout composition.

### 2.3 Responsive layout

React Native has no CSS media queries. Use `useWindowDimensions` and a small
breakpoint helper:

- `compact`: width `< 600`, phone portrait and narrow split-screen.
- `medium`: width `600-839`, large phones, small tablets, landscape phones.
- `expanded`: width `>= 840`, tablets and desktop-like windows.

Layout defaults:

- Compact screens use 16 px horizontal gutters.
- Medium and expanded screens use 24 px gutters.
- Poster grids should compute columns from available width and minimum card
  width, not hardcode web column counts.
- Detail pages should stack poster, metadata, and actions on compact screens;
  medium/expanded can use poster-left, content-right hero layouts.
- Avoid text or controls over the top safe area, home indicator, camera cutouts,
  or Android gesture/navigation areas.

### 2.4 Safe areas and system chrome

Wrap the app in `SafeAreaProvider` and use `SafeAreaView` from
`react-native-safe-area-context`. Apply edges intentionally:

- Root app shell: top, left, right, and bottom.
- Fullscreen video: draw behind system chrome only when the player controls
  explicitly account for insets.
- Bottom tab bar and audio mini-player: include bottom inset padding.
- Sheets and dialogs: include bottom inset padding and enough drag/tap clearance.

Coordinate status bar color with the active theme: dark background in dark mode,
light background in light mode, and poster/backdrop screens should still keep
status bar icons readable.

### 2.5 Accessibility

Accessibility is a core requirement for the mobile app.

- Use semantic native components where possible.
- Set `accessibilityRole` on custom controls (`button`, `link`, `tab`, `search`,
  `imagebutton`, `switch`, `adjustable`, etc.).
- Set `accessibilityLabel` on icon-only buttons, poster cards, dismiss buttons,
  tabs, and media controls.
- Use `accessibilityState` for selected tabs, disabled controls, checked toggles,
  busy loading regions, and expanded/collapsed panels.
- Use `accessibilityHint` only when the result of the action is not obvious from
  the label.
- Use `accessibilityLiveRegion` for Android state changes such as loading,
  empty, and error announcements. Use `AccessibilityInfo.announceForAccessibility`
  where an iOS announcement is needed.
- Maintain a minimum 44 x 44 touch target. Use `hitSlop` to enlarge small icon
  buttons without changing layout.
- Keep every action available by touch and screen reader. Do not hide critical
  actions behind hover or visual-only gestures.
- Support external keyboard focus where reasonable, especially for search,
  playback controls, forms, and tablet layouts. Reuse `ring` for focused outlines.

VoiceOver and TalkBack differ. Test both before considering the mobile UI done.

---

## 3. Mobile App UI & UEX

### 3.1 Navigation model

The web app uses a persistent left sidebar. Phones should use a bottom-tab
navigation model with a More area:

- **Home**
- **Movies**
- **Music**
- **Search**
- **More**

`More` contains lower-frequency destinations and account actions:

- TV Shows
- Photos
- Settings
- Notifications
- Account / profile
- Logout

Tablet layouts may add a left navigation rail when width is expanded, but the
route structure should remain compatible with the bottom-tab model. Preserve the
same six product destinations from the web client: Home, Movies, TV Shows, Music,
Photos, and Settings.

### 3.2 App shell

Authenticated content should have one app shell with:

- A safe-area-aware header when the screen needs title, search, notifications,
  theme toggle, or contextual actions.
- A bottom tab bar on compact/medium widths.
- An optional navigation rail on expanded tablet widths.
- A persistent global audio mini-player that sits above the bottom tab bar and
  respects the bottom safe area.

Login remains a no-chrome screen. Use a full-screen visual background or strong
surface treatment, a centered form card on larger screens, and a single-column
form on compact screens.

### 3.3 Home

Home is a dashboard with a welcome/continue area and stacked horizontal media
rails:

- Watch rooms
- Latest movies
- Latest albums
- Movies in theaters

On mobile, horizontal rails should be `FlatList` instances with stable item sizes,
snap-friendly spacing, and accessible card labels. Keep enough poster/title text
visible without requiring long-press or hover.

### 3.4 Movies

The movies index uses a header, stats, actions menu, and three tabs:

- All Movies
- Genres
- Playlists

Mobile implementation:

- Use a segmented control or tab row beneath the screen header.
- Use a responsive poster grid with two columns on most phones, more on tablets.
- Keep sort and filter actions in a clearly labeled menu or sheet.
- Use pagination or explicit "Load more" behavior matching API constraints; do
  not introduce infinite scroll unless the data contract changes.

Movie detail is the richest mobile screen:

- Backdrop image with a bottom gradient/scrim.
- Poster, title, tagline, metadata chips, genres, and hero actions.
- Primary Play action, Watch/Watched, Like, and More menu.
- Sections for cast, chapters, details, videos, and production companies.

On compact screens, stack the poster and metadata. On tablets, use a two-column
hero with poster left and content right. Keep Play reachable without scrolling
past long descriptions.

### 3.5 Music

The music index uses four tabs:

- Musicians
- Albums
- Tracks
- Playlists

Mobile implementation:

- Musicians: circular-image grid/list hybrid depending on width.
- Albums: square-cover grid.
- Tracks: virtualized `SectionList` or `FlatList` with A-Z section headers.
- Playlists: card/list layout with empty-state CTA.

Album and musician detail screens should follow the backdrop + hero + list
pattern. Track rows must keep Play, Like, and More actions accessible. If an
action is visually hidden until row focus on larger screens, it must remain
available to screen readers and become visible on press/focus.

### 3.6 Search

Search is a first-class bottom-tab destination on mobile.

Before any query, show a focused search field and a concise empty prompt. Results
use five tabs:

- All
- Movies
- Albums
- Musicians
- Tracks

The All tab stacks result sections with counts and "See all" actions. Category
tabs use the same media cards and track rows as the source screens. Preserve the
web default page size of `SEARCH_PER_PAGE = 24` unless the API changes.

### 3.7 Settings, notifications, and account

Settings uses the same categories as the web client:

- General
- Account
- Libraries
- Playback
- Users, admin-only

On phones, present settings as a list of categories that drill into detail
screens. On tablets, a two-pane settings layout is acceptable.

Notifications should be reachable from `More` and optionally from a header bell:

- Badge count polls every 30 seconds.
- Full notification list fetches when the panel or screen opens.
- Unread rows are tinted and include a visible unread dot.
- Each row supports mark-read behavior and a separate dismiss action.
- Failures should preserve stale content and show a non-blocking error state.

Prefer a full screen or bottom sheet on phones. A small popover is too cramped for
notification rows and separate dismiss actions.

### 3.8 Media cards

All media cards share:

- Rounded `radius.xl` surface or poster frame.
- `border` or over-media scrim.
- Stable aspect ratio.
- Clear title and secondary metadata.
- Pressed state that gently dims or scales.
- Accessibility label that names the media and action.

Card anatomy:

- **Movie card**: 2:3 poster, bottom black gradient, white title/year, Play
  affordance available from the card action or contextual menu.
- **Album card**: square cover, title and artist below, Play starts the global
  audio player while card navigation opens album detail.
- **Musician card**: circular thumbnail, centered name, album/track counts.
- **Track row**: title, artist/album context, duration when available, Play,
  Like, and More actions.

Use `Image` or the app's chosen native image component with explicit dimensions
or aspect ratio. Avoid layout shift when posters load.

### 3.9 Media playback

Movie playback should use a native player capable of direct playback, HLS
transcode streams, subtitles, audio-track selection, seeking, media-session
metadata, and lock-screen/remote controls. Do not implement playback in a WebView
unless there is a deliberate product decision.

Mobile player UX:

- Fullscreen video surface.
- Top chrome with title and Back.
- Bottom chrome with progress, elapsed/remaining time, rewind, play/pause,
  forward, quality indicator, subtitles/audio settings, and fullscreen/orientation
  handling where applicable.
- Controls appear on tap and media-key input, then auto-hide after idle.
- Resume dialog offers Resume and Start from beginning.
- Persist progress on the same cadence and thresholds as the web client.

The global audio player is persistent:

- Mini-player docked above tabs.
- Cover, title, artist, previous/play/next, and progress.
- Expanded full-screen player with queue and volume/output controls.
- Video pages pause or suppress audio-player controls when needed.

### 3.10 Loading, empty, and error states

Build shared mobile primitives even though the web currently inlines many states:

- `IglooLoading`: route-level pending screen and grid/list-matched skeletons.
- `IglooEmpty`: icon, title, optional description, optional CTA.
- `IglooError`: inline error surface with retry action.

Guidelines:

- Skeletons must match final card/list dimensions to avoid jumpy layout.
- Loading indicators need `accessibilityRole="progressbar"` or an appropriate
  busy state.
- Empty and error states should announce meaningful changes to screen readers.
- Mutation failures should use a toast/snackbar pattern that remains accessible
  and does not block the main task.

---

## 4. Web-to-Mobile Mapping

| Web pattern | React Native mobile equivalent |
|-------------|--------------------------------|
| CSS custom properties | Typed theme objects + React context |
| Tailwind utility classes | `StyleSheet.create` + style arrays |
| `bg-card`, `text-card-foreground` | `colors.card`, `colors.cardForeground` |
| `.dark` class | persisted `ThemeMode` state, defaulting to dark |
| `bg-primary/90` | alpha helper such as `withAlpha(colors.primary, 0.9)` |
| `rounded-xl` | `borderRadius: radii.xl` |
| `border-border` | `borderColor: colors.border`, often with `StyleSheet.hairlineWidth` |
| `shadow-lg` | platform shadow/elevation token |
| `gap-6`, `px-6`, `py-6` | numeric spacing tokens |
| `transition-* duration-200` | animated opacity/transform using `durations.standard` |
| `motion-reduce:` | shared `AccessibilityInfo` reduce-motion hook |
| `focus-visible:ring-*` | focused/selected outline using `colors.ring` |
| `group-hover` reveals | touch-visible controls, press state, focus state, or contextual menu |
| `line-clamp-2` | `numberOfLines={2}` and `ellipsizeMode="tail"` |
| `aspect-2/3` | explicit width + computed height, or `aspectRatio: 2 / 3` |
| `bg-linear-to-t from-black/90` | absolute overlay with native gradient component or solid scrim |
| Header search form | dedicated Search tab plus contextual search field |
| Sidebar | bottom tabs on phones, optional rail on tablets |
| Popover menus | action sheets, bottom sheets, dialogs, or tablet popovers |

Things that do not translate directly:

- Hover-only actions. Every action must be touchable and screen-reader reachable.
- Browser localStorage. Use the mobile app's storage layer while keeping the
  `igloo-theme` key semantics.
- Web focus rings. Use native focus/selected/pressed states; reserve visible
  outlines for keyboard, TV-like remote, tablet, and accessibility focus.
- CSS media queries. Use `useWindowDimensions`.
- CSS gradients. Use a native gradient implementation or a simple scrim when a
  dependency is not justified.

---

## 5. Styling Issues and Mobile Guardrails

These web-side issues are relevant because a mobile app should avoid copying the
same drift risks:

1. **No single machine-readable token source.** The web tokens are duplicated
   across CSS, boot styles, the anti-flash script, and theme constants. A mobile
   app should not create a fifth unsynchronized token source without an explicit
   synchronization plan.

2. **Spacing and typography are not exported tokens on web.** Mobile should define
   numeric spacing and type scales from the start so native styles do not become
   a scattered set of magic numbers.

3. **Over-media colors are partly raw.** Black/alpha overlays and white poster
   text are valid for media art, but document them as deliberate exceptions.

4. **Rendered toast/snackbar contrast needs coverage.** If mobile implements
   toast/snackbar states, test the actual rendered foreground/background
   combinations, not just abstract token pairs.

5. **Hidden hover actions are unsafe on mobile.** Web hover reveals must become
   touch-visible, focus-visible, or contextual actions.

Recommended improvements for a mobile build:

- Create a shared token package or generated token artifact before both web and
  mobile evolve independently.
- Add mobile contrast tests for token pairs and rendered primitives.
- Add snapshot or visual-regression coverage for compact, medium, and expanded
  layouts.
- Test VoiceOver and TalkBack flows for login, search, media cards, playback,
  notifications, and settings.
- Test reduced-motion behavior with OS-level settings enabled.
