# React Doctor Performance Report

Scan date: June 16, 2026

## Source Commands

```bash
bun run doctor
bun x react-doctor@0.5.5 --category Performance --json --yes
```

The package script and scoped JSON scan both exit with code `1` when diagnostics are present. That exit code is expected for this snapshot.

## Summary

- Total diagnostics: **28**
- Errors: **16**
- Warnings: **12**
- Affected files: **21**
- Score: **51 / 100 (Critical)**

## Performance Findings

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/AlbumCard.tsx:41:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/DeleteMovieDialog.tsx:34:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/DeleteWatchRoomDialog.tsx:60:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/SpotifyAlbumPicker.tsx:50:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/SpotifyAlbumPicker.tsx:78:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/TmdbMoviePicker.tsx:92:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement without a catch clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/TrackItem.tsx:93:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/components/VideoPlayer.tsx:114:42`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerExpression) Handle Import expressions

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/context/AudioPlayerContext.tsx:119:7`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/context/AudioPlayerContext.tsx:179:7`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/_auth/movies/$id/play.tsx:406:7`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/_auth/music/album.$id.tsx:249:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/_auth/music/index.tsx:835:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/_auth/music/index.tsx:869:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/_auth/settings/libraries.lazy.tsx:323:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `error` `todo` React Compiler doesn't support this syntax  
  Location: `src/routes/login.lazy.tsx:39:5`  
  Message: This component misses React Compiler's automatic memoization & re-renders more than it should. Rewrite the flagged code so the compiler can optimize it.  
  Help: Todo: (BuildHIR::lowerStatement) Handle TryStatement with a finalizer ('finally') clause

- `warning` `rendering-usetransition-loading` Loading useState forces extra render  
  Location: `src/components/AudioPlayer.tsx:69:37`  
  Message: This adds an extra render because useState for "isLoading" re-renders just for the loading flag, so if it's a state change & not a data fetch, use useTransition instead  
  Help: Replace with `const [isPending, startTransition] = useTransition()`, which skips the extra render for the loading flag

- `warning` `js-tosorted-immutable` Spread copy before sort()  
  Location: `src/components/MoviesInTheaters.tsx:13:14`  
  Message: This wastes work because [...array].sort() copies the array just to sort it, so use array.toSorted() to sort without the extra copy (ES2023)  
  Help: Use `array.toSorted()` (ES2023) instead of `[...array].sort()` so you sort without copying the array first

- `warning` `rerender-lazy-ref-init` Ref initializer runs on every render  
  Location: `src/context/AudioPlayerContext.tsx:67:61`  
  Message: useRef(new Map()) rebuilds this value on every render & throws it away.  
  Help: Initialize the ref lazily so expensive values are not rebuilt and discarded on every render.

- `warning` `rerender-lazy-ref-init` Ref initializer runs on every render  
  Location: `src/context/AudioPlayerContext.tsx:68:64`  
  Message: useRef(new Map()) rebuilds this value on every render & throws it away.  
  Help: Initialize the ref lazily so expensive values are not rebuilt and discarded on every render.

- `warning` `js-combine-iterations` Chained array iterations  
  Location: `src/context/AudioPlayerContext.tsx:131:27`  
  Message: This loops over your list twice because .filter().map() makes two passes, so do it in one pass with .reduce() or a for...of loop  
  Help: Combine `.map().filter()` style chains into one pass with `.reduce()` or a `for...of` loop, so you only loop over the list once

- `warning` `no-flush-sync` flushSync skips View Transitions  
  Location: `src/hooks/useElementVirtualizer.ts:2:10`  
  Message: `flushSync` forces an immediate update, which skips View Transitions and concurrent rendering.  
  Help: flushSync forces an immediate update that skips View Transitions and concurrent rendering. Use startTransition for updates that are not urgent.

- `warning` `js-hoist-intl` Intl formatter rebuilt each call  
  Location: `src/lib/format.ts:72:10`  
  Message: This is slow because new Intl.NumberFormat() rebuilds on every call inside a function, so move it to the top of the file, or wrap it in useMemo  
  Help: Move `new Intl.NumberFormat(...)` to the top of the file or wrap it in `useMemo`. Building one is slow, so don't redo it on every call

- `warning` `js-tosorted-immutable` Spread copy before sort()  
  Location: `src/lib/format.ts:130:10`  
  Message: This wastes work because [...array].sort() copies the array just to sort it, so use array.toSorted() to sort without the extra copy (ES2023)  
  Help: Use `array.toSorted()` (ES2023) instead of `[...array].sort()` so you sort without copying the array first

- `warning` `js-tosorted-immutable` Spread copy before sort()  
  Location: `src/lib/playback-recommendation.ts:16:18`  
  Message: This wastes work because [...array].sort() copies the array just to sort it, so use array.toSorted() to sort without the extra copy (ES2023)  
  Help: Use `array.toSorted()` (ES2023) instead of `[...array].sort()` so you sort without copying the array first

- `warning` `js-combine-iterations` Chained array iterations  
  Location: `src/routes/_auth/movies/in-theaters.$id.lazy.tsx:64:48`  
  Message: This loops over your list twice because .filter().map() makes two passes, so do it in one pass with .reduce() or a for...of loop  
  Help: Combine `.map().filter()` style chains into one pass with `.reduce()` or a `for...of` loop, so you only loop over the list once

- `warning` `rerender-lazy-ref-init` Ref initializer runs on every render  
  Location: `src/routes/_auth/movies/index.tsx:610:34`  
  Message: useRef(new Map()) rebuilds this value on every render & throws it away.  
  Help: Initialize the ref lazily so expensive values are not rebuilt and discarded on every render.

- `warning` `js-tosorted-immutable` Spread copy before sort()  
  Location: `src/routes/_auth/music/playlist.$id.tsx:236:22`  
  Message: This wastes work because [...array].sort() copies the array just to sort it, so use array.toSorted() to sort without the extra copy (ES2023)  
  Help: Use `array.toSorted()` (ES2023) instead of `[...array].sort()` so you sort without copying the array first
