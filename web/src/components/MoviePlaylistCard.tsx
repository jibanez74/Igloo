import { Link } from "@tanstack/react-router";
import { ListVideo } from "lucide-react";
import {
  CARD_MEDIA_HOVER_CLASS,
  CARD_SURFACE_CLASS,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { cn } from "@/lib/utils";
import type { MoviePlaylistSummaryType } from "@/types";

type MoviePlaylistCardProps = {
  playlist: MoviePlaylistSummaryType;
};

export default function MoviePlaylistCard({ playlist }: MoviePlaylistCardProps) {
  const { id, name, movie_count, cover_image, is_owner } = playlist;
  const coverUrl = getMediaImageUrl(unwrapString(cover_image));
  const movieNoun = movie_count === 1 ? "movie" : "movies";

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, "min-w-0 p-4")}
    >
      <Link
        to="/movies/playlist/$id"
        params={{ id: id.toString() }}
        className="block focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden focus-visible:ring-inset"
        aria-label={`${name}, ${movie_count} ${movieNoun}`}
      >
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-muted">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt=""
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
            />
          ) : (
            <div className="flex size-full items-center justify-center bg-linear-to-br from-muted via-muted to-primary/10">
              <ListVideo className="size-10 text-primary/30" aria-hidden="true" />
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
            {movie_count} {movieNoun}
          </p>
        </div>
      </Link>
    </article>
  );
}
