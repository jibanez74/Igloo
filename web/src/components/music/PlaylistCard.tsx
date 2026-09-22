import { Link } from "@tanstack/react-router";
import { ListMusic } from "lucide-react";
import {
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import type { PlaylistSummaryType } from "@/types";
import { formatDuration, pluralize } from "@/lib/format";
import { cn } from "@/lib/utils";

type PlaylistCardProps = {
  playlist: PlaylistSummaryType;
};

export default function PlaylistCard({ playlist }: PlaylistCardProps) {
  const { id, name, track_count, total_duration, cover_image, is_owner } =
    playlist;
  const coverUrl = getMediaImageUrl(unwrapString(cover_image));

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, "p-4")}
    >
      <Link
        to="/music/playlist/$id"
        params={{ id: id.toString() }}
        className={cn("block", FOCUS_VISIBLE_RING_CLASS, "focus-visible:ring-inset")}
        aria-label={`${name}, ${pluralize(track_count, "track")}, ${formatDuration(total_duration)}`}
      >
        {/* Playlist cover - square with aspect-square to prevent CLS */}
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-muted">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt=""
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
            />
          ) : (
            <div className="flex size-full items-center justify-center bg-linear-to-br from-muted via-muted to-primary/10">
              <ListMusic className="size-10 text-primary/30" aria-hidden="true" />
            </div>
          )}

          {/* Owner badge */}
          {is_owner && (
            <div className="absolute top-2 right-2 rounded-full bg-primary/90 px-2 py-0.5 text-xs font-medium text-primary-foreground">
              Owner
            </div>
          )}
        </div>

        {/* Playlist info */}
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-foreground">{name}</h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {pluralize(track_count, "track")}
            {total_duration > 0 && ` · ${formatDuration(total_duration)}`}
          </p>
        </div>
      </Link>
    </article>
  );
}
