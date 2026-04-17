# Video Buffering Indicator Work Plan

## Goal

Add a clear buffering indicator to the shared video player so users can tell when playback is waiting on media data, especially during HLS playback and chapter jumps, without breaking direct streaming, fullscreen controls, watch-room sync, or accessibility.

## Why This Should Live In The Shared Player

The right implementation point is `web/src/components/VideoPlayer.tsx`.

- It is already the shared playback surface for both:
  - `web/src/routes/_auth/movies/$id/play.tsx`
  - `web/src/routes/_auth/watch-rooms/$id.lazy.tsx`
- It already owns the HLS.js setup, native video element lifecycle, subtitle attachment, start-time handling, and native playback error bridge.
- Putting buffering state in the shared player keeps the behavior consistent across direct stream, native HLS, and HLS.js playback.
- It also avoids pushing more transient playback UI state into the route components, which are already doing a lot of work.

## Current State

From the current implementation:

- `VideoPlayer.tsx` renders the `<video>` element and handles source setup, but it does not track buffering or waiting state.
- The movie route already handles HLS rebasing and resume behavior via `pendingAutoPlayOnLoad`, which is important for chapter jumps.
- The watch-room route uses the same shared player and controls playback through WebSocket sync events.
- The project already has reusable UI and accessibility primitives we can lean on:
  - `web/src/components/ui/spinner.tsx`
  - `web/src/components/LiveAnnouncer.tsx`
- The frontend is already on `react@19.2.x` with the React Compiler enabled in `web/vite.config.ts`.

## Recommended Implementation

### 1. Add Shared Buffering State To `VideoPlayer`

Introduce local buffering state in `web/src/components/VideoPlayer.tsx`.

Recommended internal state shape:

- `isBuffering`: source-of-truth media waiting state
- `showBufferingIndicator`: optional presentation state if we decide to add a short anti-flicker delay
- `playbackExpected` or `resumeExpected`: only if needed to bridge HLS source swaps where the player is about to auto-resume

This state should stay inside `VideoPlayer` so route-level rerenders stay minimal.

### 2. Drive Buffering From Media Events, Not HLS.js Internals

Use the video element events as the primary truth source.

Recommended event model:

Set buffering `true` on:

- `waiting`
- `stalled`
- `seeking` when playback is expected and the seek is likely to require new data
- `loadstart` only when playback is expected, not on every idle source load

Set buffering `false` on:

- `playing`
- `canplay`
- `pause`
- `ended`
- `error`
- `seeked` if we decide the seek path needs an explicit clear

Important nuance:

- We should not blindly show buffering on every `loadstart`, because the movie page currently does not auto-play on first render. Showing a spinner before the user asks to play would feel wrong.
- We should also avoid showing buffering while the user intentionally pauses playback.

### 3. Cover HLS Chapter Jump / Rebase Windows

The HLS-specific edge case is the gap between a chapter jump or rebase and the moment playback resumes.

Recommended approach:

- Keep the main buffering logic in `VideoPlayer`.
- Add one small parent-to-player signal only if needed to cover resume-after-reload flows cleanly.

Recommended prop shape:

- `playbackExpected?: boolean`

How it would be used:

- In `web/src/routes/_auth/movies/$id/play.tsx`, derive this from current playback intent:
  - `playing`
  - `pendingAutoPlayOnLoad`
  - optionally a tiny local "play requested" flag if QA shows a gap before `waiting` fires on first play
- In `web/src/routes/_auth/watch-rooms/$id.lazy.tsx`, we can likely start without a dedicated prop and rely on media events first.
- If watch-room playback shows the same source-swap gap during sync-driven play, wire the same prop there too.

Recommendation:

- Start with the prop available in `VideoPlayer`, use it in the movie playback route in the first pass, and only extend it to watch rooms if QA shows a real gap.

### 4. Render A Non-Blocking Buffering Overlay

Add an overlay inside the existing relative player container in `VideoPlayer.tsx`.

Recommended UI:

- centered `Spinner`
- visible text such as `Buffering video...`
- semi-transparent dark backdrop so the user understands playback is waiting, but the current frame remains partially visible
- `pointer-events-none` so it never blocks:
  - click-to-play behavior in fullscreen movie playback
  - existing controls
  - focus interactions

Recommended placement:

- Inside the inner wrapper that already contains the `<video>` element
- Layered above the video, but below route-level chrome if the current z-index stack already expects that

Recommended styling goals:

- keep it visually consistent with the current player theme
- no large layout shifts
- simple opacity transition only

### 5. Accessibility Plan

This work should improve feedback without creating noisy announcements.

Recommended accessibility behavior:

- The visual overlay should include real text, not spinner-only UI.
- The overlay container should expose a status message to assistive tech.
- Add `aria-busy="true"` to the player region while buffering.
- Prefer a polite announcement strategy over assertive interruption.

Recommendation on announcements:

- First pass: expose the visible buffering text in a `role="status"` region or a lightweight local polite live region inside `VideoPlayer`.
- Avoid repeatedly firing assertive announcements on every short rebuffer.
- If repeated buffering becomes chatty in screen readers, add a small delay before announcing while still showing the visual overlay immediately.

### 6. Keep The Public Playback Contract Stable

This task should not change:

- stream URLs
- HLS server behavior
- chapter selection behavior
- watch-room sync protocol
- direct stream playback

The plan is intentionally UI-only on the frontend side.

## Concrete File Plan

### `web/src/components/VideoPlayer.tsx`

Primary work item.

Planned changes:

- Add internal buffering state.
- Register stable media event handlers with `useEffectEvent`.
- Decide whether a minimal `playbackExpected` prop is needed.
- Render the buffering overlay within the player frame.
- Keep existing HLS.js, subtitle, start-seek, and error behavior intact.
- Ensure the overlay is hidden on pause, ended, and fatal error states.

### `web/src/routes/_auth/movies/$id/play.tsx`

Likely small integration work.

Planned changes:

- If needed, pass `playbackExpected` into `VideoPlayer`.
- Derive that value from the existing playback state, especially:
  - `playing`
  - `pendingAutoPlayOnLoad`
- Keep all current chapter seeking, HLS rebasing, watch-progress persistence, and fullscreen logic unchanged.

### `web/src/routes/_auth/watch-rooms/$id.lazy.tsx`

Probably unchanged in the first pass, or only lightly updated.

Planned changes if needed:

- Pass the same optional `playbackExpected` prop if watch-room testing shows a gap during sync-driven play or rebuffering.
- Do not move buffering UI ownership into the route unless we uncover a watch-room-specific need.

### `web/src/test/video-player-buffering.test.tsx`

Add a focused component test file for the new player behavior.

This is the cleanest place to test buffering because existing watch-room tests mock `VideoPlayer`.

## Detailed Event Strategy

Recommended first-pass logic:

1. `waiting` or `stalled` while playback is expected:
   - show buffering
2. `playing` or `canplay`:
   - hide buffering
3. `pause`, `ended`, or `error`:
   - hide buffering
4. `src` changes:
   - reset buffering state cleanly
5. HLS rebase / auto-resume:
   - if `playbackExpected` is true during a source swap, allow the overlay to stay visible until `canplay` or `playing`

Decision point to validate during implementation:

- Whether `seeking` should immediately show buffering, or only `waiting` should do that.

Recommendation:

- Start with `waiting` and `stalled` as the hard trigger.
- Add `seeking` only if HLS chapter jumps feel visually silent before `waiting` arrives.
- This keeps the first pass lower risk and reduces spinner flicker on already-buffered seeks.

## React 19 Evaluation

Yes, we can use React 19 features here, but only the ones that actually help this problem.

### Good Fit

`useEffectEvent`

- The codebase already uses it in `VideoPlayer`, `AudioPlayer`, trailer playback, and watch-room sync handlers.
- It is a strong fit for media event listeners because it gives us stable event wiring without forcing more dependencies into effects.
- Recommendation: use it for the buffering event handlers.

React Compiler

- The project already has `babel-plugin-react-compiler` enabled.
- Recommendation: rely on the compiler and avoid adding manual `useMemo` and `useCallback` unless profiling proves a real hotspot.

### Available But Not A Good Fit For The Core Buffering Flag

`startTransition` / `useTransition`

- Buffering feedback is urgent UI.
- We should not lower its priority or allow it to lag behind the actual playback state.
- This is not a good fit for the overlay itself.

Possible limited use:

- If we later add non-critical secondary UI updates tied to buffering, such as analytics-driven side panels or extra informational chrome, those could be transitioned separately.

`useDeferredValue`

- Not recommended for the buffering indicator.
- Deferring the state would intentionally make the indicator late, which is the opposite of what we want.

Possible limited use:

- Not for the player overlay, but it remains useful in search/filter UIs like the existing watch-room invite dialog pattern.

`useOptimistic`, `useActionState`, `use`

- Not relevant for this task.
- The buffering indicator is driven by media runtime state, not form actions or async server payload orchestration.

## Performance Improvements Worth Considering During This Work

These are the highest-value improvements we can evaluate while implementing the buffering indicator.

### Low-Risk Improvements To Do As Part Of This Task

Keep buffering state local to `VideoPlayer`

- This prevents full movie page and watch-room page rerenders every time the player enters or exits a short waiting state.
- It is the biggest clean win for both responsiveness and maintainability.

Use native media events instead of HLS.js fragment/network callbacks

- This keeps the logic transport-agnostic.
- It also avoids subscribing to high-frequency HLS internals that would add noise and more render churn.

Reuse the existing spinner component

- No new dependency
- Consistent motion language
- Minimal implementation risk

Use CSS opacity transitions only

- Avoid layout-affecting animation for the overlay.
- Keep the indicator cheap to show and hide.

### Medium-Risk Improvements To Evaluate, But Only If QA Or Profiling Justifies Them

Add a short visibility threshold for the visual overlay

- Example: show the overlay only if buffering lasts longer than 120-200 ms.
- This can reduce flash/flicker during tiny in-buffer stalls.
- Recommendation: keep the underlying buffering state immediate, and only delay the visual presentation if needed.

Throttle route-level progress updates

- The movie route currently updates route state on every `timeupdate`.
- That can rerender the whole playback page more often than needed.
- If playback UI profiling shows heavy rerenders, consider moving progress UI to a throttled update cadence, such as `requestAnimationFrame` or a modest interval.
- This should be a separate, deliberate change because it affects progress bar smoothness, watch-progress persistence, and watch-room sync feel.

Consolidate media event handling in `VideoPlayer`

- Right now the player uses a mix of inline DOM props and effect-based logic.
- If the buffering work starts to spread event handling across too many inline props, it may be worth consolidating the media lifecycle handlers into stable internal listeners.
- Recommendation: only do this if the buffering implementation becomes messy. Do not refactor just for style.

## Test Plan

### Automated Tests

Add `web/src/test/video-player-buffering.test.tsx` with focused component coverage.

Recommended cases:

1. shows the overlay after a buffering event while playback is expected
2. hides the overlay on `playing`
3. hides the overlay on `canplay`
4. hides the overlay on `pause`
5. hides the overlay on `ended`
6. hides the overlay on `error`
7. resets correctly when `src` changes
8. if `playbackExpected` is added, keeps the overlay visible across an HLS-style source reload until the player becomes ready again

Recommended test style:

- Render `VideoPlayer` directly with a real `videoRef`
- Fire media events on the actual `<video>` element
- Keep HLS.js out of the unit test unless a very small mock is needed
- Focus on DOM and accessibility behavior, not implementation details

### Manual QA

Validate these flows in Chrome after implementation:

1. movie playback page, direct stream:
   - no regression
   - no buffering overlay while paused
2. movie playback page, HLS stream:
   - pressing play shows buffering feedback if the stream takes time to start
   - overlay clears once playback starts
3. chapter jump during active HLS playback:
   - overlay appears while the player is waiting to resume
   - overlay clears once playback resumes
4. fullscreen playback:
   - overlay is visible and centered
   - overlay does not block clicks or controls
5. watch room:
   - remote play and rebuffering still show the indicator correctly if the room enters a waiting state
6. accessibility:
   - screen readers get a sensible buffering status
   - repeated short rebuffers are not excessively noisy

## Risks And How To Avoid Them

### Risk: Spinner Shows While The User Has Not Asked To Play

Mitigation:

- Do not treat every initial `loadstart` as buffering.
- Gate buffering on actual waiting/playback intent.

### Risk: Spinner Flickers During Tiny Seeks

Mitigation:

- Start with `waiting` and `stalled` as the primary triggers.
- Add `seeking` only if needed.
- Consider a small presentation delay only if QA sees flicker.

### Risk: Overlay Blocks Existing Playback Interactions

Mitigation:

- Use `pointer-events-none`.
- Keep the overlay inside the existing player container.
- Verify fullscreen click-to-play behavior after implementation.

### Risk: Accessibility Gets Noisy

Mitigation:

- Prefer polite announcements.
- Avoid assertive repeated messaging for short stalls.
- Test with the existing `LiveAnnouncer` pattern in mind, but do not over-announce by default.

## Recommended Delivery Order

1. Add internal buffering state and overlay to `VideoPlayer.tsx`
2. Wire the movie route with a minimal `playbackExpected` prop only if needed for HLS resume/rebase coverage
3. Add the focused `VideoPlayer` component tests
4. Manually verify direct stream, HLS startup, chapter jumping, fullscreen, and watch-room playback
5. Only after QA, decide whether to add:
   - `seeking` as a trigger
   - a short anti-flicker delay
   - watch-room-specific `playbackExpected` wiring

## Recommendation

Implement this as a shared `VideoPlayer` enhancement first, with a small optional parent signal for HLS rebase windows if testing shows the plain media-event approach is not enough.

That gives us:

- consistent behavior across movie playback and watch rooms
- minimal API churn
- good accessibility
- low risk to existing playback logic
- room to layer in small performance refinements without turning the change into a rewrite
