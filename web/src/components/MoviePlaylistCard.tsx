import { Link } from "@tanstack/react-router";
import { ListVideo } from "lucide-react";
import {
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
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
      className={cn(
        CARD_INTERACTIVE_SURFACE_CLASS,
        "group relative min-w-0 overflow-hidden rounded-xl border border-slate-800 bg-slate-900 p-4 hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20",
      )}
    >
      <Link
        to="/movies/playlist/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-amber-400 focus:outline-none focus:ring-inset"
        aria-label={`${name}, ${movie_count} ${movieNoun}`}
      >
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-slate-800">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt=""
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
            />
          ) : (
            <div className="flex size-full items-center justify-center bg-linear-to-br from-slate-700 via-slate-800 to-amber-900/20">
              <ListVideo className="size-10 text-amber-200/30" aria-hidden="true" />
            </div>
          )}
          {is_owner && (
            <div className="absolute top-2 right-2 rounded-full bg-amber-500/90 px-2 py-0.5 text-xs font-medium text-slate-900">
              Owner
            </div>
          )}
        </div>
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-white">{name}</h3>
          <p className="mt-0.5 text-xs text-slate-400">
            {movie_count} {movieNoun}
          </p>
        </div>
      </Link>
    </article>
  );
}
