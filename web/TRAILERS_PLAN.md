# Trailers Route Implementation Plan

## Overview

Create a generic, reusable trailer route that can be used across different media types (movies and TV shows) using TanStack Router's route masking feature. This route will display trailers in a full-screen player while keeping the original page URL visible in the browser.

## Goals

1. **Reusable**: Single trailer route that works for any media type (movies, TV shows)
2. **Route Masking**: URL stays on the originating page (e.g., `/movies/in-theaters/123`) while the trailer plays
3. **Clean UX**: Full-screen trailer experience with easy navigation back
4. **Future-proof**: Ready to support movies, TV shows, and other media categories
5. **Type-safe**: Search params validated with Zod

## Route Structure

### New Route

```
/_auth/trailer
```

- **Path**: `/trailer`
- **File**: `web/src/routes/_auth/trailer.tsx` (NOT lazy loaded)
- **Data**: Passed via validated search params

### Why NOT Lazy Loaded?

The trailer route is NOT lazy loaded because:

1. It's a critical user interaction that should load instantly
2. The component is relatively small
3. Avoids loading spinner when user clicks "Play Trailer"

### Why Search Params Instead of URL Params?

Using search params (`/trailer?mediaType=movie&mediaId=123`) instead of URL params (`/trailer/movie/123`):

1. More flexible for adding additional params later
2. Cleaner route masking (the search params are also masked)
3. Easier validation with Zod schemas
4. Can include return navigation info in the same search object

## Search Params Schema

```tsx
import { z } from "zod";

export const trailerSearchSchema = z.object({
  // Required: The type of media
  mediaType: z.enum(["movie", "tv"]),

  // Required: The TMDB ID of the media
  mediaId: z.coerce.number().int().positive(),

  // Optional: The route to return to when closing
  returnTo: z.string().optional(),
});

export type TrailerSearch = z.infer<typeof trailerSearchSchema>;
```

### Route Definition

```tsx
import { createFileRoute } from "@tanstack/react-router";
import { trailerSearchSchema } from "./trailer.schema"; // or inline

export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  component: TrailerPage,
});
```

## Route Masking Strategy

When navigating to the trailer from any details page, use route masking:

```tsx
// From a movie page
<Link
  to="/trailer"
  search={{
    mediaType: "movie",
    mediaId: movie.id,
    returnTo: "/movies/in-theaters/$id",
  }}
  mask={{
    to: "/movies/in-theaters/$id",
    params: { id: String(movie.id) },
  }}
>
  Play Trailer
</Link>

// From a TV show page (future)
<Link
  to="/trailer"
  search={{
    mediaType: "tv",
    mediaId: show.id,
    returnTo: "/tv/popular/$id",
  }}
  mask={{
    to: "/tv/popular/$id",
    params: { id: String(show.id) },
  }}
>
  Play Trailer
</Link>
```

This means:

- **Actual route**: `/trailer?mediaType=movie&mediaId=123&returnTo=/movies/in-theaters/$id`
- **Displayed URL**: `/movies/in-theaters/123` (search params are also masked!)

## Implementation Steps

### Step 1: Create the Trailer Route

Create `web/src/routes/_auth/trailer.tsx`:

```tsx
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { z } from "zod";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
// import { tvDetailsQueryOpts } from '@/lib/query-opts' // future

const trailerSearchSchema = z.object({
  mediaType: z.enum(["movie", "tv"]),
  mediaId: z.coerce.number().int().positive(),
  returnTo: z.string().optional(),
});

export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  component: TrailerPage,
});

function TrailerPage() {
  const { mediaType, mediaId, returnTo } = Route.useSearch();
  const navigate = useNavigate();

  // Fetch based on media type
  const query =
    mediaType === "movie"
      ? movieDetailsQueryOpts(mediaId)
      : movieDetailsQueryOpts(mediaId); // TODO: tvDetailsQueryOpts when implemented

  const { data } = useQuery(query);

  const media = data?.data?.movie; // TODO: handle TV shows
  const trailer = media?.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube"
  );

  const handleClose = () => {
    if (returnTo) {
      navigate({ to: returnTo, params: { id: String(mediaId) } });
    } else {
      navigate({ to: "/" });
    }
  };

  // ... render full-screen player
}
```

Key features:

- Full-screen YouTube player component
- Fetches media details based on `mediaType` search param
- Uses Zod validation for type-safe search params
- Extracts trailer from `media.videos.results`
- Close button navigates to `returnTo` route
- Escape key to close
- **Auto-navigate back when video ends** (see below)

### Step 2: Update Movie Details Pages

Update each movie details page to use the new trailer route:

1. `web/src/routes/_auth/movies/in-theaters.$id.lazy.tsx`
2. (Future) `web/src/routes/_auth/movies/upcoming.$id.lazy.tsx`
3. (Future) `web/src/routes/_auth/movies/popular.$id.lazy.tsx`

Changes needed:

- Import `Link` from `@tanstack/react-router`
- Import `buttonVariants` from button component
- Replace `Button` + `YoutubePlayer` with `Link` using route masking
- Remove `useState` for `trailerOpen`
- Remove `YoutubePlayer` import

### Step 3: (Future) Update TV Show Details Pages

When TV shows are implemented:

1. `web/src/routes/_auth/tv/popular.$id.lazy.tsx`
2. `web/src/routes/_auth/tv/on-air.$id.lazy.tsx`
3. etc.

Same pattern as movies, just with `mediaType: "tv"`

### Step 4: Create Trailer Player Component (Optional Refactor)

Consider extracting the trailer player into a reusable component:

`web/src/components/TrailerPlayer.tsx`:

- Receives `videoKey`, `title`, `onClose`
- Full-screen layout with close button
- ReactPlayer integration
- Keyboard controls (Escape to close)

## File Changes Summary

| File                                                   | Action | Description                           |
| ------------------------------------------------------ | ------ | ------------------------------------- |
| `web/src/routes/_auth/trailer.tsx`                     | Create | New trailer route (not lazy)          |
| `web/src/routes/_auth/movies/in-theaters.$id.lazy.tsx` | Modify | Use Link with route masking           |
| `web/src/components/YoutubePlayer.tsx`                 | Keep   | May still be useful for other dialogs |

## Example Usage

### From In-Theaters Movie Details

```tsx
<Link
  to='/trailer'
  search={{
    mediaType: "movie",
    mediaId: movie.id,
    returnTo: "/movies/in-theaters/$id",
  }}
  mask={{
    to: "/movies/in-theaters/$id",
    params: { id: String(movie.id) },
  }}
  className={buttonVariants({ variant: "accent", size: "lg" })}
>
  <i className='fa-solid fa-play' aria-hidden='true' />
  Play Trailer
</Link>
```

### From Upcoming Movies (Future)

```tsx
<Link
  to='/trailer'
  search={{
    mediaType: "movie",
    mediaId: movie.id,
    returnTo: "/movies/upcoming/$id",
  }}
  mask={{
    to: "/movies/upcoming/$id",
    params: { id: String(movie.id) },
  }}
  className={buttonVariants({ variant: "accent", size: "lg" })}
>
  <i className='fa-solid fa-play' aria-hidden='true' />
  Play Trailer
</Link>
```

### From Popular TV Shows (Future)

```tsx
<Link
  to='/trailer'
  search={{
    mediaType: "tv",
    mediaId: show.id,
    returnTo: "/tv/popular/$id",
  }}
  mask={{
    to: "/tv/popular/$id",
    params: { id: String(show.id) },
  }}
  className={buttonVariants({ variant: "accent", size: "lg" })}
>
  <i className='fa-solid fa-play' aria-hidden='true' />
  Play Trailer
</Link>
```

## Validation Benefits

Using Zod with TanStack Router provides:

1. **Type Safety**: Search params are fully typed in components
2. **Runtime Validation**: Invalid params are caught before component renders
3. **Coercion**: `z.coerce.number()` automatically converts string "123" to number 123
4. **Default Values**: Can provide defaults for optional params
5. **Error Handling**: Router can show error UI for invalid params

```tsx
// This is fully typed!
const { mediaType, mediaId, returnTo } = Route.useSearch();
//      ^-- 'movie' | 'tv'
//                   ^-- number
//                            ^-- string | undefined
```

## Keyboard Shortcuts and Accessibility

The trailer player must support comprehensive keyboard shortcuts for screen reader users and keyboard navigation, following the same pattern as the AudioPlayer component.

### Keyboard Shortcuts

| Key                 | Action                   | Notes                     |
| ------------------- | ------------------------ | ------------------------- |
| `Space` or `K`      | Play/Pause               | Matches YouTube shortcuts |
| `ArrowLeft` or `J`  | Seek backward 10 seconds |                           |
| `ArrowRight` or `L` | Seek forward 10 seconds  |                           |
| `ArrowUp`           | Volume up 10%            |                           |
| `ArrowDown`         | Volume down 10%          |                           |
| `M`                 | Mute/Unmute              |                           |
| `F`                 | Toggle fullscreen        | Browser fullscreen API    |
| `Home` or `0`       | Restart video            |                           |
| `Escape`            | Close trailer            | Navigate back to origin   |

### Implementation Pattern

Follow the AudioPlayer keyboard handling pattern:

```tsx
useEffect(
  () => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Skip when user is typing in an input field
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      // Skip when modifier keys are pressed (allow browser shortcuts)
      if (e.ctrlKey || e.metaKey || e.altKey) {
        return;
      }

      switch (e.key) {
        case " ":
        case "k":
        case "K":
          e.preventDefault();
          togglePlay();
          break;
        case "ArrowLeft":
        case "j":
        case "J":
          e.preventDefault();
          seekBackward(10);
          break;
        case "ArrowRight":
        case "l":
        case "L":
          e.preventDefault();
          seekForward(10);
          break;
        case "ArrowUp":
          e.preventDefault();
          increaseVolume();
          break;
        case "ArrowDown":
          e.preventDefault();
          decreaseVolume();
          break;
        case "m":
        case "M":
          e.preventDefault();
          toggleMute();
          break;
        case "f":
        case "F":
          e.preventDefault();
          toggleFullscreen();
          break;
        case "Home":
        case "0":
          e.preventDefault();
          restartVideo();
          break;
        case "Escape":
          e.preventDefault();
          handleClose();
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  },
  [
    /* dependencies */
  ]
);
```

### Screen Reader Support

1. **Dialog Role**: Container has `role="dialog"` and `aria-modal="true"`
2. **Live Region**: Announce playback state changes
   ```tsx
   <div className='sr-only' aria-live='polite' aria-atomic='true'>
     {isPlaying ? `Playing: ${title}` : `Paused: ${title}`}
   </div>
   ```
3. **Labeled Buttons**: All buttons have descriptive `aria-label` attributes
4. **Progress Slider**: Use `role="slider"` with proper ARIA attributes
   ```tsx
   <div
     role='slider'
     aria-label='Video progress'
     aria-valuenow={Math.round(currentTime)}
     aria-valuemin={0}
     aria-valuemax={Math.round(duration)}
     aria-valuetext={`${formatTime(currentTime)} of ${formatTime(duration)}`}
   />
   ```

### Focus Management

1. **Initial Focus**: Focus the play/pause button when trailer opens

   ```tsx
   useEffect(() => {
     if (playPauseButtonRef.current) {
       const timer = setTimeout(() => {
         playPauseButtonRef.current?.focus();
       }, 50);
       return () => clearTimeout(timer);
     }
   }, []);
   ```

2. **Focus Trap**: Keep focus within the trailer dialog

   ```tsx
   const handleKeyDown = (e: React.KeyboardEvent) => {
     if (e.key === "Tab") {
       const focusableElements =
         containerRef.current?.querySelectorAll<HTMLElement>(
           'button:not([disabled]), [tabindex]:not([tabindex="-1"])'
         );
       // Trap focus within dialog
     }
   };
   ```

3. **Return Focus**: When trailer closes, focus returns to the "Play Trailer" button (handled by browser navigation)

### Keyboard Shortcuts Help

Display available shortcuts on hover/focus:

```tsx
<p className='sr-only'>
  Keyboard shortcuts: Space or K to play/pause, J or left arrow to rewind 10
  seconds, L or right arrow to forward 10 seconds, up/down arrows for volume, M
  to mute, F for fullscreen, Escape to close.
</p>
```

## Auto-Navigate on Video End

When the trailer finishes playing, automatically navigate the user back to the originating page. This provides a seamless experience without requiring manual interaction.

### Implementation

Use ReactPlayer's `onEnded` callback to trigger navigation:

```tsx
function TrailerPage() {
  const { mediaId, returnTo } = Route.useSearch();
  const navigate = useNavigate();

  const handleClose = useCallback(() => {
    if (returnTo) {
      navigate({ to: returnTo, params: { id: String(mediaId) } });
    } else {
      navigate({ to: "/" });
    }
  }, [navigate, returnTo, mediaId]);

  // Called when video finishes playing
  const handleVideoEnd = useCallback(() => {
    handleClose();
  }, [handleClose]);

  return (
    <ReactPlayer
      url={videoUrl}
      playing={true}
      controls
      onEnded={handleVideoEnd}
      // ...
    />
  );
}
```

### User Experience Flow

```
1. User clicks "Play Trailer" on movie details page
2. Route navigates to /trailer (masked as /movies/in-theaters/123)
3. Trailer plays in full-screen
4. When trailer ends:
   → Automatically navigate back to /movies/in-theaters/123
   → User is back on the movie details page seamlessly
```

### Screen Reader Announcement

Announce the navigation to screen reader users:

```tsx
const handleVideoEnd = useCallback(() => {
  // Optional: announce before navigating
  // The navigation itself will announce the new page
  handleClose();
}, [handleClose]);
```

The page change will naturally be announced by the screen reader when the new page loads.

## Considerations

### Data Loading

- The trailer route uses the same query options as detail pages
- Data will likely be cached from the originating page, resulting in instant trailer display

### Accessibility Summary

- Full keyboard navigation matching YouTube shortcuts
- Screen reader announcements via ARIA live regions
- Focus management when opening/closing
- Focus trap within dialog
- Skip shortcuts when typing in inputs
- Allow browser shortcuts (Cmd+R, etc.)

### Mobile

- Touch-friendly close button
- Responsive video player sizing
- On-screen controls visible on touch

### Invalid Search Params

- If someone navigates directly to `/trailer` without valid params, Zod validation will fail
- Consider adding an `errorComponent` to the route for graceful handling

```tsx
export const Route = createFileRoute("/_auth/trailer")({
  validateSearch: trailerSearchSchema,
  component: TrailerPage,
  errorComponent: ({ error }) => (
    <div>Invalid trailer request. Please try again.</div>
  ),
});
```

## Directory Structure After Implementation

```
web/src/routes/
├── _auth/
│   ├── route.tsx
│   ├── index.lazy.tsx
│   ├── trailer.tsx                            ← NEW: Generic trailer route (not lazy)
│   ├── movies/
│   │   └── in-theaters.$id.lazy.tsx          ← Modified to use trailer route
│   ├── tv/                                    ← Future
│   │   ├── popular.$id.lazy.tsx
│   │   └── on-air.$id.lazy.tsx
│   └── music/
│       ├── route.tsx
│       ├── index.lazy.tsx
│       └── album.$id.tsx
├── login/
│   ├── route.tsx
│   └── index.lazy.tsx
└── __root.tsx
```

## Future Enhancements

1. **Multiple trailers**: If media has multiple trailers, allow selection via additional search param
2. **Trailer carousel**: Navigate between trailers of different media
3. **Picture-in-Picture**: Allow minimizing trailer while browsing
4. **Share trailer**: Deep link directly to trailer (unmasks on share)
5. **Trailer types**: Support teasers, featurettes, clips via `videoType` search param
