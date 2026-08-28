import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { showActionFailed } from "@/lib/toast-helpers";
import { Music, Play } from "lucide-react";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import { Spinner } from "@/components/ui/spinner";
import {
  CARD_ACTION_REVEAL_CLASS,
  CARD_FOCUS_WITHIN_RING_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  CARD_SURFACE_CLASS,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { cn } from "@/lib/utils";
import type { SimpleAlbumType } from "@/types";

type AlbumCardProps = {
  album: SimpleAlbumType;
  /** Replaces the musician-name line (e.g. "2024 · 8 tracks" on the musician page). */
  subtitle?: string;
};

export default function AlbumCard({ album, subtitle }: AlbumCardProps) {
  const { id, title, cover, musician } = album;
  const queryClient = useQueryClient();
  const audioPlayer = useAudioPlayerActions();
  const [isLoading, setIsLoading] = useState(false);

  const coverUrl = getMediaImageUrl(unwrapString(cover)) ?? "";
  const { showPoster: showCover, onError } = usePosterFallback(coverUrl);

  const handlePrefetch = () =>
    queryClient.prefetchQuery(albumDetailsQueryOpts(id));

  const handlePlayAlbum = async (e: React.MouseEvent<HTMLButtonElement>) => {
    e.preventDefault();
    e.stopPropagation();
    if (isLoading) return;
    setIsLoading(true);

    try {
      const data = await queryClient.fetchQuery(albumDetailsQueryOpts(id));

      if (!data.error && data.data?.tracks?.length > 0) {
        audioPlayer.playQueue(data.data.tracks, {
          cover: unwrapString(data.data.album.cover),
          title: data.data.album.title,
          musician: unwrapString(data.data.album.musician),
        });
      }
    } catch (error) {
      console.error("Failed to load album:", error);

      showActionFailed("play album", "Something went wrong. Please try again.");
    }

    setIsLoading(false);
  };

  const musicianName = unwrapString(musician);
  const infoLine = subtitle ?? musicianName;
  const cardLabel = subtitle
    ? `${title}, ${subtitle}`
    : `${title}${musicianName ? ` by ${musicianName}` : ""}`;

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, CARD_FOCUS_WITHIN_RING_CLASS)}
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <Link
        to="/music/album/$id"
        params={{ id: id.toString() }}
        className="block rounded-xl outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
        aria-label={cardLabel}
      >
        {/* Album cover: local /api/static/albums/... or external URL; fallback on load error */}
        <div className="relative aspect-square bg-muted">
          {showCover ? (
            <img
              src={coverUrl}
              alt={`Album cover for ${title}`}
              width={640}
              height={640}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              sizes="(min-width: 1024px) 16.66vw, (min-width: 768px) 25vw, (min-width: 640px) 33.33vw, 50vw"
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              onError={onError}
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Music className="size-10 text-muted-foreground" aria-hidden="true" />
            </div>
          )}

          {/* Play button overlay - appears on hover/focus */}
          <div
            className={cn(
              CARD_OVERLAY_REVEAL_CLASS,
              "absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100",
            )}
            aria-hidden="true"
          />
        </div>

        {/* Album info */}
        <div className="min-h-17 p-3">
          <h3 className="line-clamp-2 text-sm/tight font-semibold text-foreground">
            {title}
          </h3>
          {infoLine && (
            <p className="mt-1 truncate text-xs text-muted-foreground">
              {infoLine}
            </p>
          )}
        </div>
      </Link>

      {/* Play button - positioned over the cover image */}
      <button
        type="button"
        onClick={handlePlayAlbum}
        aria-disabled={isLoading}
        className={cn(
          CARD_ACTION_REVEAL_CLASS,
          "absolute top-1/2 left-1/2 flex size-12 -translate-x-1/2 -translate-y-[calc(50%+1rem)] scale-90 items-center justify-center rounded-full bg-primary text-primary-foreground opacity-0 shadow-lg shadow-black/30 outline-hidden group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-primary/90 focus-visible:scale-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background aria-disabled:opacity-50",
        )}
        aria-label={`Play ${cardLabel}`}
      >
        {isLoading ? (
          <Spinner className="size-5" />
        ) : (
          <Play className="size-5 fill-current" aria-hidden="true" />
        )}
      </button>
    </article>
  );
}
