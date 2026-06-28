# Web UI Remediation Plan — unify the styling system & adopt the igloo palette

> Companion to [`design-system.md`](./design-system.md) (finding #1) and
> [`igloo-theme.ts`](./igloo-theme.ts). This is the **phased plan** to fix the
> dual-styling-system problem and re-skin the web app to the cool *igloo* identity.
> Each phase is independently shippable and verifiable; do them in order.

## The problem (finding #1, recap)

The web app has **two parallel styling systems** and the token one is **not switched on**:

- `web/src/assets/styles.css` defines shadcn OKLCH tokens with a `:root` (light) block and a
  `.dark` override block, gated by `@custom-variant dark (&:is(.dark *))`.
- **The `.dark` class is never added to `<html>`** (`web/index.html` ships
  `data-app-ready="false"`; `web/src/AppBoot.tsx:14` only sets `data-app-ready="true"`).
- So tokens resolve to **light** values and the **25 `dark:`-prefixed utilities** in
  components never apply. The app only *looks* dark because `boot.css` paints navy and
  components hardcode dark utilities.

Current hardcoded surface area (grep of `web/src`):

| Pattern | Count | | Pattern | Count |
| --- | --- | --- | --- | --- |
| `bg-slate-*` | 409 | | `amber-*` (all) | 449 |
| `text-slate-*` | 514 | | `ring-amber-*` | 98 |
| `border-slate-*` | 197 | | files touching amber | 66 |
| `dark:` variants | 25 | | | |

## End state (goal)

- **One source of truth for color:** semantic tokens in `styles.css`, holding the **igloo
  dark** palette (glacier primary + sparing amber aurora). A future re-skin = edit tokens.
- Components read **tokens** (`bg-background`, `bg-card`, `text-foreground`, `bg-primary`,
  `ring-ring`, `bg-aurora`) instead of raw `slate-*` / `amber-*`.
- **One focus-ring color** (glacier) everywhere.
- Accessibility unchanged (contrast ≥ AA per `design-system.md` §5.3; focus visible).

## Guiding principles

1. **Smallest reversible steps.** Land each phase separately; compare against the
   `docs/design/` "before" screenshots after each.
2. **Behavior/a11y first.** Never regress focus order, labels, live regions, or contrast.
   Re-run the Playwright a11y suites each phase.
3. **Mechanical where possible.** Phase 4 is a grep-driven mapping; keep diffs reviewable by
   doing it per screen area.

---

## Phase 1 — Activate dark mode (make the existing design coherent)

**Why first:** the components were authored *for* dark mode (hence the 25 `dark:` variants and
light token defaults). Turning dark on is a ~1-line change that instantly makes tokens and
`dark:` variants agree — high signal, near-zero risk, and it surfaces any component that was
silently relying on light defaults.

**Change:** add the `dark` class to `<html>`. Cleanest is alongside the existing boot flag in
`web/src/AppBoot.tsx`:

```ts
const root = document.documentElement;
root.classList.add("dark");          // tokens + dark: variants now active
root.setAttribute("data-app-ready", "true");
```

(Or hardcode `class="dark"` in `web/index.html`. Doing it in `AppBoot` keeps a single place to
later add a theme toggle.)

**Verify:** `bun run dev`; spot-check default `Button`/`Card`/`Input`/`Select`, dialogs, and the
outline-button variant (`button.tsx:16` `dark:bg-slate-800/90`) — they should now render their
intended dark look. Compare to `docs/design/`. Run `bun run lint && bun run build`.

---

## Phase 2 — Re-skin the tokens to the igloo palette

Make the *active* (now-dark) tokens hold the §5.3 values. Convert the §5.3 hex to OKLCH (keep
the existing OKLCH style) and set them in the **`.dark` block** of `web/src/assets/styles.css`:

- `--background` → `canvas` `#0A1322`; `--card`/`--popover` → `surfaceRaised`/`surfaceOverlay`;
  `--secondary`/`--muted` → `surface`; `--foreground` → `#F8FAFC`; `--muted-foreground` →
  `#8094AE`.
- **`--primary` → glacier `#38BDF8`**, `--primary-foreground` → `#08131F`.
- `--border`/`--input` → `borderStrong`/subtle; **`--ring` → glacier `#38BDF8`**.
- `--destructive` → `#F87171`; sidebar* → glacier-tinted equivalents.
- **Add new semantic tokens** and expose them as utilities via `@theme inline`
  (so `bg-aurora`, `text-aurora`, `bg-success`, etc. work):
  ```css
  @theme inline { --color-aurora: var(--aurora); --color-success: var(--success); /* … */ }
  .dark { --aurora:<amber>; --success:<green>; --accent-teal:<teal>; }
  ```
- **`boot.css`**: keep the navy base; recolor the warm amber radial glow
  (`rgba(251,191,36,0.14)`) to a subtle glacier/aurora wash; set `html` background to the
  `canvas` value. Update `<meta name="theme-color">` in `index.html` to match.

**Verify:** the app shifts cool; any component still reading tokens looks igloo. `lint`/`build`.
Hardcoded `slate-*`/`amber-*` still present — handled next.

---

## Phase 3 — Unify focus ring, accent, and card chrome

- **Focus ring:** replace `ring-amber-400` / `ring-cyan-400` with the token `ring-ring`
  (glacier). 98 `ring-amber-*` usages — do as part of the per-area passes below or in one
  focused sweep.
- **Button accent:** in `web/src/components/ui/button.tsx`, change `accent` / `accent-pill`
  from `bg-amber-500 text-slate-900` to the glacier token (`bg-primary text-primary-foreground`).
  Introduce a separate `aurora` variant **only** for genuinely warm spots (ratings / "in
  theaters") so amber stays intentional, not default.
- **Shared media surface:** extract a `MediaCard` (or shared class set) from the duplicated
  chrome in `MovieCard.tsx` / `AlbumCard.tsx` / `PlaylistCard.tsx`, built on the existing
  `CARD_*` constants (`web/src/lib/constants.ts`). Convert the amber hover glow
  (`hover:shadow-amber-500/20`) to a glacier glow.

**Verify:** focus rings are one color and clearly visible (a11y); media cards behave
identically but cool. Playwright a11y suites pass.

---

## Phase 4 — Migrate hardcoded utilities to tokens (per screen area)

Mechanical, grep-driven. Do it **one area at a time** (sidebar/shell → home → movies → movie
detail → player → music → settings), comparing each against the `docs/design/` baseline. Use
this mapping (adjust per context):

| Current hardcoded | Token replacement |
| --- | --- |
| `bg-slate-900` (page/sidebar/inset) | `bg-background` |
| `bg-slate-900`/`bg-slate-800` (panels/cards) | `bg-card` / `bg-muted` |
| `bg-slate-800` (inputs/hover) | `bg-muted` / `bg-accent` |
| `text-white` | `text-foreground` |
| `text-slate-300` / `text-slate-400` | `text-muted-foreground` |
| `border-slate-800/50` / `border-slate-700` | `border-border` |
| `bg-amber-500` `hover:bg-amber-400` (CTA) | `bg-primary` `hover:bg-primary/90` |
| `bg-amber-500` (ratings / in-theaters) | `bg-aurora` (keep warm, intentional) |
| `ring-amber-400` / `ring-cyan-400` | `ring-ring` |
| `shadow-amber-500/20` (card glow) | glacier glow utility/token |
| `text-amber-400` (active nav icon) | `text-primary` |

Keep amber **only** in the explicitly-warm spots (e.g. `InTheatersCard` score badge). Each
area is a small, reviewable PR.

**Verify (per area):** visual diff vs baseline, `lint`, `build`, `test`, a11y suites.

---

## Phase 5 — Guardrails & docs

- **Lint/CI guard:** add an ESLint `no-restricted-syntax` (or a tiny test) that flags new raw
  `slate-*` / `amber-*` (allowlist the sanctioned aurora spots) so the codebase doesn't drift
  back. Mirrors the existing `--max-warnings 0` discipline.
- **Contrast check** in CI alongside the current a11y Playwright assertions.
- **Remove dead CSS:** delete the now-unused `:root` light block (or keep it intentionally for
  Phase 6). If kept, ensure it is a *real* light igloo theme, not stock shadcn.
- **Flip the docs:** in `design-system.md`, move the igloo palette from "proposed" to
  "current"; refresh `docs/design/` screenshots as the new baseline.

---

## Phase 6 (optional) — Real light theme

Only if a light/daytime mode is wanted. Reintroduce a light igloo token set in `:root`, add a
theme toggle (persist to localStorage / user setting), set `.dark` accordingly in `AppBoot`,
and test both themes for contrast. Treat as a separate effort.

---

## Risk & rollback

- Phases 1–2 are config-level and trivially revertible (one file each).
- Phase 4 is the bulk of the churn but is purely class substitution; the `docs/design/`
  baseline is the regression oracle. Split by area so any single revert is small.
- **Accessibility is the gate** at every phase: focus visibility, contrast (≥ AA), and the
  existing Playwright suites (`web/e2e/*`) must stay green.

## Suggested sequencing

1. Phase 1 (1 small PR) → 2 (1 PR) → 3 (2–3 PRs) → 4 (≈7 area PRs) → 5 (1–2 PRs).
2. Land 1–3 quickly (they make the app coherent and on-brand with low risk); spread Phase 4
   across normal feature work using the mapping table.
