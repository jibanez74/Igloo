import { Link } from "@tanstack/react-router";
import { ListMusic, ListVideo } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
} from "@/lib/constants";
import { formatDuration, pluralize } from "@/lib/format";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { unwrapString } from "@/lib/nullable";
import { cn } from "@/lib/utils";
import type { MoviePlaylistSummaryType, PlaylistSummaryType } from "@/types";

/** What differs between a music and a movie playlist card, by `content_type`. */
const PLAYLIST_KINDS = {
  track: { to: "/music/playlist/$id", icon: ListMusic },
  movie: { to: "/movies/playlist/$id", icon: ListVideo },
} as const;

type PlaylistCardProps = {
  playlist: PlaylistSummaryType | MoviePlaylistSummaryType;
};

/** Square-cover card for a music or movie playlist (design-system §3.2). */
export default function PlaylistCard({ playlist }: PlaylistCardProps) {
  const { id, name, cover_image, is_owner } = playlist;
  const { to, icon: PlaceholderIcon } = PLAYLIST_KINDS[playlist.content_type];
  const coverUrl = getMediaImageUrl(unwrapString(cover_image)) ?? "";
  const { showPoster: showCover, onError } = usePosterFallback(coverUrl);

  // The visible line joins with " · ", the accessible name with commas; both
  // leave out a zero duration rather than read "0s".
  const details =
    playlist.content_type === "track"
      ? [
          pluralize(playlist.track_count, "track"),
          ...(playlist.total_duration > 0
            ? [formatDuration(playlist.total_duration)]
            : []),
        ]
      : [pluralize(playlist.movie_count, "movie")];

  return (
    <article className={cn(CARD_SURFACE_CLASS, "min-w-0 p-4")}>
      <Link
        to={to}
        params={{ id: id.toString() }}
        className={cn("block", FOCUS_VISIBLE_RING_CLASS, "focus-visible:ring-inset")}
        aria-label={[name, ...details].join(", ")}
      >
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-muted">
          {showCover ? (
            <img
              src={coverUrl}
              alt=""
              width={640}
              height={640}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              sizes="(min-width: 1024px) 20vw, (min-width: 768px) 25vw, (min-width: 640px) 33.33vw, 50vw"
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              onError={onError}
            />
          ) : (
            <div className="flex size-full items-center justify-center bg-linear-to-br from-muted via-muted to-primary/10">
              <PlaceholderIcon className="size-10 text-primary/30" aria-hidden="true" />
            </div>
          )}
          {is_owner && (
            <div className="absolute top-2 right-2 rounded-full bg-primary/90 px-2 py-0.5 text-xs font-medium text-primary-foreground">
              Owner
            </div>
          )}
        </div>
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-foreground">{name}</h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {details.join(" · ")}
          </p>
        </div>
      </Link>
    </article>
  );
}

// Authored beside the card it mirrors (design-system §3.4): the same padded
// square cover, then bars sized to the name and details lines.
export function PlaylistCardSkeleton() {
  return (
    <div
      className={cn(
        "rounded-xl border border-border bg-card p-4",
        MOTION_LOADING_STATE_CLASS,
      )}
    >
      <div className="mb-3 aspect-square w-full rounded-lg bg-muted" />
      <div className="flex h-5 items-center justify-center">
        <div className="h-4 w-3/4 rounded-sm bg-muted" />
      </div>
      <div className="mt-0.5 flex h-4 items-center justify-center">
        <div className="h-3 w-1/2 rounded-sm bg-muted" />
      </div>
    </div>
  );
}
