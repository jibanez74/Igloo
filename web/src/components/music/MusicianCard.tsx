import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { User } from "lucide-react";
import { musicianDetailsQueryOpts } from "@/lib/query-opts";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_FOCUS_WITHIN_RING_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { cn } from "@/lib/utils";
import type { SimpleMusicianType } from "@/types";

type MusicianCardProps = {
  musician: SimpleMusicianType;
};

const pluralize = (count: number, noun: string) =>
  `${count} ${noun}${count === 1 ? "" : "s"}`;

export default function MusicianCard({ musician }: MusicianCardProps) {
  const { id, name, thumb, album_count, track_count } = musician;
  const queryClient = useQueryClient();

  const thumbUrl = getMediaImageUrl(unwrapString(thumb)) ?? "";
  const { showPoster: showThumb, onError } = usePosterFallback(thumbUrl);

  const handlePrefetch = () =>
    queryClient.prefetchQuery(musicianDetailsQueryOpts(id));

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, CARD_FOCUS_WITHIN_RING_CLASS, "p-4")}
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <Link
        to="/music/musician/$id"
        params={{ id: id.toString() }}
        className="block rounded-xl outline-hidden"
        aria-label={`${name}, ${pluralize(album_count, "album")}, ${pluralize(track_count, "track")}`}
      >
        {/* Musician thumbnail - circular; fallback to User icon on load error */}
        <div className="relative mx-auto mb-3 aspect-square w-full max-w-32 overflow-hidden rounded-full bg-muted">
          {showThumb ? (
            <img
              src={thumbUrl}
              alt={name}
              width={256}
              height={256}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              sizes="128px"
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              onError={onError}
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <User className="size-10 text-muted-foreground" aria-hidden="true" />
            </div>
          )}
        </div>

        {/* Musician info */}
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-foreground">{name}</h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {pluralize(album_count, "album")} · {pluralize(track_count, "track")}
          </p>
        </div>
      </Link>
    </article>
  );
}
