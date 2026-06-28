# Visual baseline — "before" screenshots

Captured from the running app to document the **current** UI before any igloo re-skin.
These are the regression oracle for the remediation work in
[`../design-remediation-plan.md`](../design-remediation-plan.md).

## Capture conditions

- Date: 2026-06-28. Desktop viewport 1440×900 @2x (mobile shot 390×844 @2x).
- Backend `make dev` + web `bun run dev` (Vite :3000 → API :8080), logged in as the default
  admin. Library scanned from the real movie/music dirs (~316 movies).
- Script: a short Playwright run (system Chrome) — login, then navigate + full-page screenshot.
- **Caveat:** `DOWNLOAD_IMAGES=false`, so posters/covers come from the on-demand TMDB proxy and
  many had **not cached yet at capture time** → empty gradient placeholders on grids. Layout,
  spacing, typography, and the accent language are still fully represented (see the movie-detail
  and "Now Playing" shots, where artwork loaded).
- **No secrets:** the Settings shot shows all credential fields **masked**; URL fields empty.

## Files

| File | Screen |
| --- | --- |
| `01-login.png` | Login (ice/penguin backdrop, amber logo + Sign-in button, amber focus ring) |
| `02-home.png` | Home dashboard — Recently Added Movies/Albums, Now Playing in Theaters |
| `03-movies.png` | Movies library grid |
| `04-movie-detail.png` | Movie detail (LOTR) — hero backdrop, cast, chapters, details, extras |
| `05-music.png` | Music library |
| `06-settings.png` | Settings → General (toggles, selects, masked credentials) |
| `07-search.png` | Search results (`q=lord`) |
| `08-home-mobile.png` | Home at phone width (collapsed sidebar) |

## What the baseline confirms (matches the audit)

- **Warm amber accent everywhere it should become glacier:** logo tile, active-nav highlight &
  icon, primary CTAs (Sign in / Play / Save Settings), focus rings, the circular play button +
  **amber hover-glow** on focused media cards, rating badges, genre pills, section icons.
- **Cool surfaces** (navy/slate) + white text — the only "cold" cue today.
- **Scattered cyan** (e.g. Settings toggle icons) — the inconsistency noted in the audit.
- Coherent layout system: 2:3 posters, square covers, consistent grids, generous radii, subtle
  motion. These carry over unchanged; only the palette/accent should shift cool.
