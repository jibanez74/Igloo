# React Doctor top 10 issues

Generated from:

```sh
cd web
bun run doctor
bun x react-doctor@0.5.5 --yes --verbose
```

Run date: 2026-06-23

React Doctor score: **53 / 100 Critical**

Summary from verbose output:

- **247 total warnings**
- **Security:** 2
- **Bugs:** 38
- **Performance:** 8
- **Accessibility:** 87
- **Maintainability:** 112

Ranking criteria: prioritize source issues that are likely to affect users, accessibility, playback reliability, stale UI state, or React Compiler compatibility. Treat generated-file findings and broad style-only findings as lower priority unless they hide real source issues.

## 1. Exclude ignored generated artifacts from React Doctor scans

React Doctor reported both security warnings from ignored Playwright report files:

- `web/playwright-report/trace/sw.bundle.js`
- `web/playwright-report/trace/assets/defaultSettingsView-BNmKHKpQ.js`

These files are ignored by `.gitignore` and are not app source. Fix the scan scope before using the score as a quality gate, otherwise the report includes false-positive security noise and inflated totals.

Recommended fix: configure React Doctor, or the command that invokes it, to exclude ignored generated directories such as `web/playwright-report/`, `web/test-results/`, `web/dist/`, and `web/node_modules/`.

## 2. Confirm and fix stale watch-progress cleanup behavior

Finding:

- `web/src/hooks/useMovieWatchProgressSaver.ts:56`

The unmount cleanup reads `currentTimeRef.current` and `durationRef.current` while flushing watch progress. The code comments say this is intentional so the latest playback position is saved, but this is playback-sensitive and should be verified carefully.

Recommended fix: keep the "flush latest position on unmount" behavior, but make the ref dependency intent explicit in a way React Doctor and ESLint can both understand. If the current pattern is retained, document this as a reviewed false positive in the tool config rather than leaving repeated diagnostics.

## 3. Fix derived state in edit/settings forms

Findings include:

- `web/src/components/EditMovieDialog.tsx:143`
- `web/src/routes/_auth/settings/index.lazy.tsx:238`
- `web/src/routes/_auth/settings/users.lazy.tsx:642-644`
- `web/src/routes/_auth/settings/playback.lazy.tsx:208`
- `web/src/routes/_auth/settings/libraries.lazy.tsx:218`

Several forms copy props into local state. Some settings pages already try to resync when backing data changes, but dialog forms such as the user and movie editors can still show stale values if the component stays mounted while the selected record changes.

Recommended fix: reset form state when the backing record identity changes, or key dialogs by the edited entity ID so React remounts the form for a different entity. Keep dirty-form preservation intentional on settings pages.

## 4. Add missing cache invalidation or document no-op mutations

Findings:

- `web/src/routes/_auth/settings/users.lazy.tsx:189`
- `web/src/routes/_auth/settings/account.lazy.tsx:255`

The reset-password and update-password mutations do not update or invalidate React Query cache data. Password changes may not need cache invalidation, but the current pattern is inconsistent with nearby mutations and should be reviewed.

Recommended fix: for mutations that change visible cached data, invalidate the relevant query keys on success. For password-only mutations that truly do not affect cached UI data, add a narrow suppression or configuration note after review.

## 5. Add accessible labels to unlabeled controls

Findings:


- `web/src/components/EditMovieDialog.tsx:294`
- `web/src/components/PlaylistFormDialog.tsx:254`
- `web/src/routes/_auth/settings/account.lazy.tsx:868`

The textarea findings appear to be associated with visible labels, so they need verification. The hidden username input in account settings may be a browser autocomplete helper, but React Doctor still sees an unlabeled control.

Recommended fix: verify each control with the rendered DOM. Add `aria-label`, `aria-labelledby`, or correct label association where missing. For intentionally hidden autocomplete fields, use the smallest accessible pattern that keeps password manager behavior intact.

## 6. Replace clickable static player wrapper with semantic interaction

Finding:

- `web/src/components/YoutubePlayer.tsx:58`

The YouTube dialog uses a static `div` with keyboard handling to stop propagation. Even if the file is currently reported as unused, this pattern can confuse assistive tech if the component is reintroduced.

Recommended fix: move keyboard handling to a semantic dialog/player container pattern, or add the correct role and keyboard semantics if the wrapper is intentionally interactive. If the component is truly dead, delete it instead.

## 7. Review dynamic subtitle/caption handling for media elements

Findings:

- `web/src/components/AudioPlayer.tsx:519`
- `web/src/components/VideoPlayer.tsx:263`
- `web/src/test/watch-room-page.test.tsx:182`

`VideoPlayer` dynamically appends a subtitle `<track>` when `subtitleTrack` is present, so the static diagnostic is incomplete. The audio player has no equivalent caption or transcript track.

Recommended fix: verify expected subtitle and caption behavior for each media mode. Prefer declarative `<track>` rendering where practical for video. For audio, decide whether lyrics/transcripts/captions are supported; if not, document and suppress only after the accessibility decision is explicit.

## 8. Reduce effect-driven event logic in virtualized music lists

Findings include:

- `web/src/routes/_auth/music/index.tsx:681`
- `web/src/routes/_auth/music/index.tsx:698`
- `web/src/routes/_auth/music/index.tsx:750-751`
- `web/src/routes/_auth/music/playlist.$id.tsx:711`
- `web/src/routes/_auth/search/index.lazy.tsx:587`

React Doctor flagged event-like behavior handled through `useEffect`. The virtualized music list calls `requestNextPage()` from intersection and rendered-item effects, which can trigger extra renders or duplicate pagination requests if state changes rapidly.

Recommended fix: centralize infinite-scroll triggering so each page request is guarded once, preferably at the intersection boundary or virtualizer callback boundary. Keep explicit guards for `hasNextPage` and `isFetchingNextPage`.

Follow-up: before changing this area, verify whether React 19 APIs or virtualizer-provided callbacks can simplify the pagination trigger without increasing duplicate fetch risk.

## 9. Simplify media control state updates that batch poorly

Findings include:

- `web/src/components/VolumeControl.tsx:51`
- `web/src/hooks/useVideoFullscreen.ts:54`
- `web/src/hooks/useIdleControls.ts:30`

Media controls update multiple related state values from effects and event listeners. These are small components, but they sit in high-frequency playback UI where extra renders are more noticeable.

Recommended fix: group related state with reducers or single state objects where updates are logically coupled. For `useIdleControls`, avoid scheduling state resets from effects unless the delayed reset is required for correctness.

## 10. Triage high-volume maintainability findings before broad refactors

Large groups:

- `no-multi-comp`: 74 findings
- `no-giant-component`: 12 findings
- `prefer-module-scope-pure-function`: 7 findings
- `react-compiler-no-manual-memoization`: 3 findings

These are real maintainability signals, but many would require broad route/component refactors. The highest-risk files are playback and dense route modules, especially `AudioPlayer`, movie playback, music index, settings pages, and playlist pages.

Recommended fix: avoid a single cleanup PR. Start with low-risk React Compiler cleanup, such as removing redundant `useCallback` where behavior is unchanged, then split large components only when touching those screens for functional work.

## Lower-priority notes

- `src/components/ui/label.tsx:7` is likely a false positive against the reusable shadcn Label primitive, not a specific unlabeled form field.
- `src/components/ui/button.tsx:68` flags a standard shadcn-style `buttonVariants` export. Move variants only if Fast Refresh behavior is actually affected.
- `role="status"` to `<output>` suggestions appear frequently; validate semantics before doing mechanical replacements.
- Gray text on colored background has 28 findings and should be fixed during visual/accessibility polish, but it is less urgent than unlabeled controls and playback/state correctness.
