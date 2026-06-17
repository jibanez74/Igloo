# React Doctor Bugs

- Generated: 2026-06-16
- Working directory: `web/`
- Script run: `bun run doctor`
- Verbose follow-up: `bun x react-doctor@0.5.5 --yes --verbose`
- Note: `bun run doctor` reported `error: script "doctor" exited with code 1` after printing the findings summary.

## Summary

- Total bug findings: 85
- Errors: 3
- Warnings: 82

## Rule Counts

- `no-adjust-state-on-prop-change` — State synced to a prop inside an effect (3 errors)
- `button-has-type` — Button missing explicit type (43 warnings)
- `no-cascading-set-state` — Multiple setState calls in one effect (4 warnings)
- `no-derived-state` — Derived value copied into state (2 warnings)
- `no-derived-useState` — Prop derived into useState (5 warnings)
- `no-event-handler` — Event logic handled in an effect (9 warnings)
- `no-initialize-state` — State initialized from a mount effect (4 warnings)
- `no-pass-data-to-parent` — Data passed to parent via effect (1 warning)
- `no-prop-callback-in-effect` — Parent kept in sync with a callback effect (2 warnings)
- `no-reset-all-state-on-prop-change` — All state reset on prop change (1 warning)
- `prefer-use-effect-event` — Effect re-subscribes on a changing callback (1 warning)
- `prefer-useReducer` — Many related useState calls (8 warnings)
- `query-mutation-missing-invalidation` — Mutation without cache invalidation (2 warnings)

## Detailed Findings

### `no-adjust-state-on-prop-change` — State synced to a prop inside an effect

- Severity: error
- Occurrences: 3
- Message: This effect adjusts state after a prop changes, so users briefly see the stale value.
- Suggested fix: Adjust the state inline during render with a `prev`-prop comparison (`if (prop !== prevProp) { setPrevProp(prop); setX(...); }`), or refactor to remove the duplicated state. Routing the adjustment through a useEffect forces an extra render with a stale UI between the two commits. See https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes
- Locations:
  - `web/src/routes/_auth/settings/index.lazy.tsx:230`
  - `web/src/routes/_auth/settings/index.lazy.tsx:231`
  - `web/src/routes/_auth/settings/playback.lazy.tsx:197`

### `button-has-type` — Button missing explicit type

- Severity: warning
- Occurrences: 43
- Message: Your users can submit the form by accident because a `<button>` with no `type` defaults to submit.
- Suggested fix: Set an explicit button `type` so plain buttons do not submit forms by accident: `type="button"`, `"submit"`, or `"reset"`.
- Locations:
  - `web/src/components/AlbumCard.tsx:123`
  - `web/src/components/AudioPlayer.tsx:532`
  - `web/src/components/AudioPlayer.tsx:549`
  - `web/src/components/AudioPlayer.tsx:605`
  - `web/src/components/AudioPlayer.tsx:617`
  - `web/src/components/AudioPlayer.tsx:636`
  - `web/src/components/AudioPlayer.tsx:675`
  - `web/src/components/AudioPlayer.tsx:710`
  - `web/src/components/AudioPlayer.tsx:722`
  - `web/src/components/AudioPlayer.tsx:740`
  - `web/src/components/AudioPlayer.tsx:768`
  - `web/src/components/AudioPlayer.tsx:780`
  - `web/src/components/MoviePlayerControls.tsx:100`
  - `web/src/components/MoviePlayerControls.tsx:110`
  - `web/src/components/MoviePlayerControls.tsx:124`
  - `web/src/components/MoviePlayerControls.tsx:153`
  - `web/src/components/TrackActionsMenu.tsx:48`
  - `web/src/components/TrackItem.tsx:149`
  - `web/src/components/TrackItem.tsx:203`
  - `web/src/components/TrackItem.tsx:237`
  - `web/src/components/ui/sidebar.tsx:267`
  - `web/src/routes/_auth/movies/$id/play.tsx:575`
  - `web/src/routes/_auth/movies/index.tsx:522`
  - `web/src/routes/_auth/music/album.$id.tsx:452`
  - `web/src/routes/_auth/music/album.$id.tsx:459`
  - `web/src/routes/_auth/music/album.$id.tsx:470`
  - `web/src/routes/_auth/music/index.tsx:846`
  - `web/src/routes/_auth/music/index.tsx:880`
  - `web/src/routes/_auth/music/index.tsx:1291`
  - `web/src/routes/_auth/music/musician.$id.tsx:301`
  - `web/src/routes/_auth/music/musician.$id.tsx:311`
  - `web/src/routes/_auth/music/playlist.$id.tsx:349`
  - `web/src/routes/_auth/music/playlist.$id.tsx:357`
  - `web/src/routes/_auth/music/playlist.$id.tsx:371`
  - `web/src/routes/_auth/music/playlist.$id.tsx:381`
  - `web/src/routes/_auth/trailer.lazy.tsx:277`
  - `web/src/routes/_auth/trailer.lazy.tsx:315`
  - `web/src/routes/_auth/trailer.lazy.tsx:392`
  - `web/src/routes/_auth/trailer.lazy.tsx:459`
  - `web/src/routes/_auth/trailer.lazy.tsx:470`
  - `web/src/routes/_auth/trailer.lazy.tsx:487`
  - `web/src/routes/_auth/trailer.lazy.tsx:500`
  - `web/src/routes/_auth/trailer.lazy.tsx:517`

### `no-cascading-set-state` — Multiple setState calls in one effect

#### Variant 1

- Severity: warning
- Occurrences: 3
- Message: 3 setState calls in one useEffect redraw your screen each time they run together.
- Suggested fix: Combine related updates in `useReducer` so one effect does not redraw the screen once per `setState` call.
- Locations:
  - `web/src/hooks/useIdleControls.ts:30`
  - `web/src/hooks/useVideoFullscreen.ts:54`
  - `web/src/routes/_auth/settings/index.lazy.tsx:225`

#### Variant 2

- Severity: warning
- Occurrences: 1
- Message: 4 setState calls in one useEffect redraw your screen each time they run together.
- Suggested fix: Combine related updates in `useReducer` so one effect does not redraw the screen once per `setState` call.
- Locations:
  - `web/src/components/VolumeControl.tsx:51`

### `no-derived-state` — Derived value copied into state

- Severity: warning
- Occurrences: 2
- Message: Storing "form" in state when you can derive it from other values costs an extra render.
- Suggested fix: Work out the value while rendering (or with useMemo if it's expensive) instead of copying it into useState through a useEffect. See https://react.dev/learn/you-might-not-need-an-effect#updating-state-based-on-props-or-state
- Locations:
  - `web/src/routes/_auth/settings/index.lazy.tsx:229`
  - `web/src/routes/_auth/settings/playback.lazy.tsx:196`

### `no-derived-useState` — Prop derived into useState

#### Variant 1

- Severity: warning
- Occurrences: 1
- Message: Your users see a stale value when prop "movie" changes because useState copies it once.
- Suggested fix: Compute the value inline so prop changes do not leave `useState` holding a stale copy.
- Locations:
  - `web/src/components/EditMovieDialog.tsx:142`

#### Variant 2

- Severity: warning
- Occurrences: 1
- Message: Your users see a stale value when prop "settings" changes because useState copies it once.
- Suggested fix: Compute the value inline so prop changes do not leave `useState` holding a stale copy.
- Locations:
  - `web/src/routes/_auth/settings/libraries.lazy.tsx:218`

#### Variant 3

- Severity: warning
- Occurrences: 3
- Message: Your users see a stale value when prop "user" changes because useState copies it once.
- Suggested fix: Compute the value inline so prop changes do not leave `useState` holding a stale copy.
- Locations:
  - `web/src/routes/_auth/settings/users.lazy.tsx:641`
  - `web/src/routes/_auth/settings/users.lazy.tsx:642`
  - `web/src/routes/_auth/settings/users.lazy.tsx:643`

### `no-event-handler` — Event logic handled in an effect

#### Variant 1

- Severity: warning
- Occurrences: 7
- Message: Faking an event handler with a prop plus a useEffect costs an extra render & runs late.
- Suggested fix: Run the side effect in the event handler that triggers it, instead of watching its state from a useEffect. See https://react.dev/learn/you-might-not-need-an-effect#sharing-logic-between-event-handlers
- Locations:
  - `web/src/components/VolumeControl.tsx:45`
  - `web/src/routes/_auth/music/index.tsx:681`
  - `web/src/routes/_auth/music/index.tsx:750`
  - `web/src/routes/_auth/music/index.tsx:751`
  - `web/src/routes/_auth/music/playlist.$id.tsx:707`
  - `web/src/routes/_auth/search/index.lazy.tsx:587` (x2)

#### Variant 2

- Severity: warning
- Occurrences: 2
- Message: Faking an event handler with state plus a useEffect costs an extra render & runs late.
- Suggested fix: Run the side effect in the event handler that triggers it, instead of watching its state from a useEffect. See https://react.dev/learn/you-might-not-need-an-effect#sharing-logic-between-event-handlers
- Locations:
  - `web/src/components/VolumeControl.tsx:143`
  - `web/src/routes/_auth/music/index.tsx:698`

### `no-initialize-state` — State initialized from a mount effect

#### Variant 1

- Severity: warning
- Occurrences: 1
- Message: Your users see an extra render with empty "coarse" because a useEffect sets its starting value.
- Suggested fix: Pass the initial value directly to useState() instead of setting it from a mount-only useEffect. For SSR hydration, prefer useSyncExternalStore().
- Locations:
  - `web/src/hooks/use-coarse-pointer.ts:14`

#### Variant 2

- Severity: warning
- Occurrences: 1
- Message: Your users see an extra render with empty "hasInitialSplash" because a useEffect sets its starting value.
- Suggested fix: Pass the initial value directly to useState() instead of setting it from a mount-only useEffect. For SSR hydration, prefer useSyncExternalStore().
- Locations:
  - `web/src/components/AppLoadingScreen.tsx:29`

#### Variant 3

- Severity: warning
- Occurrences: 1
- Message: Your users see an extra render with empty "isBrowserFullscreen" because a useEffect sets its starting value.
- Suggested fix: Pass the initial value directly to useState() instead of setting it from a mount-only useEffect. For SSR hydration, prefer useSyncExternalStore().
- Locations:
  - `web/src/routes/_auth/trailer.lazy.tsx:115`

#### Variant 4

- Severity: warning
- Occurrences: 1
- Message: Your users see an extra render with empty "prefersReducedMotion" because a useEffect sets its starting value.
- Suggested fix: Pass the initial value directly to useState() instead of setting it from a mount-only useEffect. For SSR hydration, prefer useSyncExternalStore().
- Locations:
  - `web/src/hooks/useContentFadeTransition.ts:40`

### `no-pass-data-to-parent` — Data passed to parent via effect

- Severity: warning
- Occurrences: 1
- Message: Handing data back to a parent from a useEffect costs your users an extra render.
- Suggested fix: Fetch the data in the parent and pass it down as a prop (or return it from the hook), instead of handing it back up through a prop callback in a useEffect. See https://react.dev/learn/you-might-not-need-an-effect#passing-data-to-the-parent
- Locations:
  - `web/src/hooks/useMoviePlaybackData.ts:146`

### `no-prop-callback-in-effect` — Parent kept in sync with a callback effect

#### Variant 1

- Severity: warning
- Occurrences: 1
- Message: Your parent re-renders on every local state change because this useEffect calls the prop "fetchNextPage" just to stay in sync.
- Suggested fix: Move the shared state into a Provider so both sides read the same value. Then you don't need a useEffect to keep them in sync.
- Locations:
  - `web/src/routes/_auth/music/playlist.$id.tsx:725`

#### Variant 2

- Severity: warning
- Occurrences: 1
- Message: Your parent re-renders on every local state change because this useEffect calls the prop "requestNextPage" just to stay in sync.
- Suggested fix: Move the shared state into a Provider so both sides read the same value. Then you don't need a useEffect to keep them in sync.
- Locations:
  - `web/src/routes/_auth/music/index.tsx:753`

### `no-reset-all-state-on-prop-change` — All state reset on prop change

- Severity: warning
- Occurrences: 1
- Message: Your users briefly see stale state when a prop changes because this useEffect clears all state.
- Suggested fix: Pass the prop as `key` so React resets the component for you when the prop changes, instead of clearing every state value by hand in a useEffect. See https://react.dev/learn/you-might-not-need-an-effect#resetting-all-state-when-a-prop-changes
- Locations:
  - `web/src/components/MusicianCard.tsx:24`

### `prefer-use-effect-event` — Effect re-subscribes on a changing callback

- Severity: warning
- Occurrences: 1
- Message: Your effect re-subscribes whenever "onMinimize" changes, even though it's only used inside `addEventListener`.
- Suggested fix: Wrap the callback with `useEffectEvent(callback)` (React 19+) and call it inside the sub-handler. An Effect Event always sees the latest props and state but isn't a dependency, so the effect won't re-subscribe every time the parent redraws. See https://react.dev/reference/react/useEffectEvent
- Locations:
  - `web/src/components/AudioPlayer.tsx:120`

### `prefer-useReducer` — Many related useState calls

#### Variant 1

- Severity: warning
- Occurrences: 1
- Message: 10 useState calls in "ManualTab" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/components/EditMovieDialog.tsx:138`

#### Variant 2

- Severity: warning
- Occurrences: 1
- Message: 5 useState calls in "CreateUserDialog" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/routes/_auth/settings/users.lazy.tsx:420`

#### Variant 3

- Severity: warning
- Occurrences: 1
- Message: 5 useState calls in "LibrariesSettingsForm" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/routes/_auth/settings/libraries.lazy.tsx:216`

#### Variant 4

- Severity: warning
- Occurrences: 1
- Message: 5 useState calls in "LibraryMovieDetailsContent" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/routes/_auth/movies/$id/index.tsx:136`

#### Variant 5

- Severity: warning
- Occurrences: 1
- Message: 5 useState calls in "SpotifyAlbumPicker" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/components/SpotifyAlbumPicker.tsx:31`

#### Variant 6

- Severity: warning
- Occurrences: 1
- Message: 7 useState calls in "TmdbMoviePicker" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/components/TmdbMoviePicker.tsx:39`

#### Variant 7

- Severity: warning
- Occurrences: 1
- Message: 8 useState calls in "PlayMoviePage" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/routes/_auth/movies/$id/play.tsx:127`

#### Variant 8

- Severity: warning
- Occurrences: 1
- Message: 9 useState calls in "AccountSettings" can each trigger a separate render.
- Suggested fix: Group related state in `useReducer` so one logical update does not fan out into separate renders.
- Locations:
  - `web/src/routes/_auth/settings/account.lazy.tsx:90`

### `query-mutation-missing-invalidation` — Mutation without cache invalidation

- Severity: warning
- Occurrences: 2
- Message: useMutation with no cache update leaves your users looking at stale data after it runs.
- Suggested fix: Add `onSuccess: () => queryClient.invalidateQueries({ queryKey: ['...'] })` so cached data stays in sync after the mutation
- Locations:
  - `web/src/routes/_auth/settings/account.lazy.tsx:255`
  - `web/src/routes/_auth/settings/users.lazy.tsx:189`
