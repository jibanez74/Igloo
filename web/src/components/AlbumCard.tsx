import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { showActionFailed } from "@/lib/toast-helpers";
import { Music, Play } from "lucide-react";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { Spinner } from "@/components/ui/spinner";
import {
  CARD_ACTION_REVEAL_CLASS,
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
};

export default function AlbumCard({ album }: AlbumCardProps) {
  const { id, title, cover, musician } = album;
  const queryClient = useQueryClient();
  const audioPlayer = useAudioPlayerActions();
  const [isLoading, setIsLoading] = useState(false);
  const [failedCoverUrl, setFailedCoverUrl] = useState<string | null>(null);

  const coverUrl = getMediaImageUrl(unwrapString(cover));

  const handlePrefetch = () =>
    queryClient.prefetchQuery(albumDetailsQueryOpts(id));

  const handlePlayAlbum = async (e: React.MouseEvent<HTMLButtonElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsLoading(true);

    try {
      const data = await queryClient.fetchQuery(albumDetailsQueryOpts(id));

      if (!data.error && data.data?.tracks?.length > 0) {
        audioPlayer.playAlbum(data.data.tracks, {
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
  const showCover = coverUrl && failedCoverUrl !== coverUrl;

  return (
    <article
      className={CARD_SURFACE_CLASS}
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <Link
        to="/music/album/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-ring focus:outline-none focus:ring-inset"
        aria-label={`${title}${musicianName ? ` by ${musicianName}` : ""}`}
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
              onError={() => setFailedCoverUrl(coverUrl)}
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
          {musicianName && (
            <p className="mt-1 truncate text-xs text-muted-foreground">
              {musicianName}
            </p>
          )}
        </div>
      </Link>

      {/* Play button - positioned over the cover image */}
      <button
        type="button"
        onClick={handlePlayAlbum}
        disabled={isLoading}
        className={cn(
          CARD_ACTION_REVEAL_CLASS,
          "absolute top-1/2 left-1/2 flex size-12 -translate-x-1/2 -translate-y-[calc(50%+1rem)] scale-90 items-center justify-center rounded-full bg-primary text-primary-foreground opacity-0 shadow-lg shadow-black/30 group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-primary/90 focus:scale-100 focus:opacity-100 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-none disabled:opacity-50",
        )}
        aria-label={`Play ${title}${musicianName ? ` by ${musicianName}` : ""}`}
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
