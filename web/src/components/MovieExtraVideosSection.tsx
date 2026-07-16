import { Link } from "@tanstack/react-router";
import { Film, Play } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
} from "@/lib/constants";
import { formatExtraVideoType } from "@/lib/format";
import { buildYouTubeThumbnailUrl } from "@/lib/youtube-thumb-url";
import { cn } from "@/lib/utils";
import type { LibraryMovieExtraVideoType } from "@/types";
import type { MovieExtraVideosSectionProps } from "@/types";

function ExtraVideoCard({
  video,
  returnTo,
}: {
  video: LibraryMovieExtraVideoType;
  returnTo: string;
}) {
  const thumbnailUrl = buildYouTubeThumbnailUrl(video.key);
  const { showPoster, onError } = usePosterFallback(thumbnailUrl);

  return (
    <Link
      to="/trailer"
      search={{
        videoKey: video.key,
        returnTo,
      }}
      className={cn(
        "group block touch-manipulation overflow-hidden rounded-lg border border-primary/20 bg-muted/50 transition-colors hover:border-primary/40",
        FOCUS_VISIBLE_RING_CLASS,
      )}
    >
      <div className="relative aspect-video overflow-hidden bg-muted">
        {showPoster ? (
          <img
            src={thumbnailUrl}
            alt=""
            aria-hidden="true"
            loading="lazy"
            decoding="async"
            fetchPriority="low"
            width={480}
            height={270}
            className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
            onError={onError}
          />
        ) : (
          <div className="flex size-full items-center justify-center">
            <Film className="size-8 text-muted-foreground" aria-hidden="true" />
          </div>
        )}
        <div
          className={cn(
            CARD_OVERLAY_REVEAL_CLASS,
            "absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100",
          )}
          aria-hidden="true"
        >
          <span className="flex size-12 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg shadow-black/30">
            <Play className="size-5 fill-current" aria-hidden="true" />
          </span>
        </div>
      </div>
      <div className="px-3 py-2.5 text-left text-sm">
        <span className="line-clamp-2 leading-snug font-medium text-foreground">
          {video.title}
        </span>
        <span className="mt-0.5 block text-muted-foreground">
          ({formatExtraVideoType(video.type)})
        </span>
      </div>
    </Link>
  );
}

export default function MovieExtraVideosSection({
  videos,
  movieId,
  trailerReturnTo,
}: MovieExtraVideosSectionProps) {
  if (videos.length === 0) return null;

  const returnTo = trailerReturnTo ?? `/movies/${movieId}`;

  return (
    <section
      className="mt-8 sm:mt-10"
      aria-labelledby="extra-videos-heading"
    >
      <h2
        id="extra-videos-heading"
        tabIndex={-1}
        className="mb-4 text-xl font-semibold text-foreground outline-none sm:text-2xl"
      >
        Extra Videos
      </h2>
      <p className="sr-only">
        Extra videos scroll horizontally. Use Tab to move between video links.
        On touch devices, swipe or scroll the list to see all clips.
      </p>
      <ul
        className="-mx-4 flex snap-x snap-mandatory scrollbar-thin scrollbar-thumb-primary/50 list-none gap-3 overflow-x-auto overscroll-x-contain px-4 pb-4 sm:-mx-6 sm:gap-4 sm:px-6 lg:-mx-8 lg:gap-4 lg:px-8"
        aria-label={`Extra videos, ${videos.length} clips`}
      >
        {videos.map(video => (
          <li
            key={video.id}
            className="w-[min(16rem,calc(100vw-2.5rem))] shrink-0 snap-start scroll-ms-1 scroll-me-1 sm:w-72 sm:scroll-ms-2 sm:scroll-me-2"
          >
            <ExtraVideoCard video={video} returnTo={returnTo} />
          </li>
        ))}
      </ul>
    </section>
  );
}
