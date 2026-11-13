import { Skeleton } from "@/components/ui/skeleton";
import { Link } from "@tanstack/react-router";
import { ListMusic, ListVideo } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
  TMDB_POSTER_SIZE,
} from "@/lib/constants";
import { formatDuration, pluralize } from "@/lib/format";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { unwrapString } from "@/lib/nullable";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { MoviePlaylistSummaryType, PlaylistSummaryType } from "@/types";

/**
 * What differs between a music and a movie playlist card, by `content_type`.
 * `artworkUrl` turns one `artwork` entry into an image URL: an album cover is
 * already a URL, a poster is a TMDB path. Posters take the library cards' size,
 * so the collage reuses images the All tab already loaded.
 */
const PLAYLIST_KINDS = {
  track: {
    to: "/music/playlist/$id",
    icon: ListMusic,
    artworkUrl: (cover: string) => getMediaImageUrl(cover) ?? "",
  },
  movie: {
    to: "/movies/playlist/$id",
    icon: ListVideo,
    artworkUrl: (posterPath: string) =>
      buildTmdbImageUrl(posterPath, TMDB_POSTER_SIZE),
  },
} as const;

const COLLAGE_SIZE = 4;

/**
 * The images in the card's art box: the playlist's own cover; else a 2×2
 * collage when four items have artwork; else the first item's image alone,
 * because two or three would leave holes in the grid.
 */
function playlistArtUrls(coverUrl: string, artworkUrls: string[]) {
  if (coverUrl !== "") return [coverUrl];
  if (artworkUrls.length >= COLLAGE_SIZE) return artworkUrls.slice(0, COLLAGE_SIZE);
  return artworkUrls.slice(0, 1);
}

type PlaylistCardProps = {
  playlist: PlaylistSummaryType | MoviePlaylistSummaryType;
};

/** Square-cover card for a music or movie playlist (design-system §3.2). */
export default function PlaylistCard({ playlist }: PlaylistCardProps) {
  const { id, name, cover_image, is_owner } = playlist;
  const { to, icon: PlaceholderIcon, artworkUrl } = PLAYLIST_KINDS[playlist.content_type];
  const artUrls = playlistArtUrls(
    getMediaImageUrl(unwrapString(cover_image)) ?? "",
    playlist.artwork.map(artworkUrl).filter(url => url !== ""),
  );
  // One failure state for the whole box: a collage with a hole reads as broken.
  const { showPoster: showArt, onError } = usePosterFallback(artUrls.join(" "));
  const isCollage = artUrls.length === COLLAGE_SIZE;

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
  // A playlist shared with the reader names its owner on a line of its own,
  // so a long name never pushes the counts out of view.
  const ownerLine = is_owner ? null : `By ${playlist.owner.name}`;

  return (
    <article className={cn(CARD_SURFACE_CLASS, "min-w-0 p-4")}>
      <Link
        to={to}
        params={{ id: id.toString() }}
        className={cn("block", FOCUS_VISIBLE_RING_CLASS, "focus-visible:ring-inset")}
        aria-label={[name, ...(ownerLine ? [ownerLine] : []), ...details].join(", ")}
      >
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-muted">
          {showArt && isCollage ? (
            <div
              className={cn(
                "grid size-full grid-cols-2 grid-rows-2",
                CARD_MEDIA_HOVER_CLASS,
              )}
            >
              {artUrls.map((url, slot) => (
                <img
                  // Four fixed cells: two movies can share a poster path.
                  key={slot}
                  src={url}
                  alt=""
                  width={320}
                  height={320}
                  loading="lazy"
                  decoding="async"
                  fetchPriority="low"
                  className="size-full min-h-0 object-cover"
                  onError={onError}
                />
              ))}
            </div>
          ) : showArt ? (
            <img
              src={artUrls[0]}
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
          {ownerLine && (
            <p className="mt-0.5 truncate text-xs text-muted-foreground">{ownerLine}</p>
          )}
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
      <Skeleton className="mb-3 aspect-square w-full rounded-lg" />
      <div className="flex h-5 items-center justify-center">
        <Skeleton className="h-4 w-3/4" />
      </div>
      <div className="mt-0.5 flex h-4 items-center justify-center">
        <Skeleton className="h-3 w-1/2" />
      </div>
    </div>
  );
}
