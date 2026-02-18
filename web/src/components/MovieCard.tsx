import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { Film, Play } from "lucide-react";
import { libraryMovieDetailsQueryOpts } from "@/lib/query-opts";
import type { LatestMovieType } from "@/types";

export default function MovieCard({ movie }: { movie: LatestMovieType }) {
  const { id, title, poster, year } = movie;
  const queryClient = useQueryClient();

  const handlePrefetch = () =>
    queryClient.prefetchQuery(libraryMovieDetailsQueryOpts(id));

  return (
    <article
      className="group relative min-w-0 animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 transition-all duration-300 fade-in focus-within:ring-2 focus-within:ring-cyan-400 focus-within:ring-offset-2 focus-within:ring-offset-slate-900 hover:-translate-y-1 hover:border-cyan-500/50 hover:shadow-xl hover:shadow-cyan-500/20"
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <Link
        to="/movies/$id"
        params={{ id: String(id) }}
        className="block rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 focus-visible:ring-offset-2 focus-visible:ring-offset-slate-900"
        aria-label={`${title}${year ? `, ${year}` : ""}`}
      >
        {/* Poster with 2:3 aspect ratio (standard movie poster) */}
        <div className="relative aspect-2/3 bg-slate-800">
          {poster ? (
            <img
              src={poster}
              alt=""
              width={500}
              height={750}
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Film className="size-10 text-slate-600" aria-hidden="true" />
            </div>
          )}
          {/* Overlay - appears on hover/focus */}
          <div
            className="absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100"
            aria-hidden="true"
          />
          {/* Gradient overlay for text readability */}
          <div className="absolute inset-x-0 bottom-0 h-24 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
        </div>
        {/* Movie info */}
        <div className="absolute inset-x-0 bottom-0 p-3">
          <h3 className="truncate text-sm font-semibold text-white drop-shadow-lg">
            {title}
          </h3>
          {year != null && (
            <p className="mt-0.5 text-xs text-slate-300 drop-shadow-lg">
              {year}
            </p>
          )}
        </div>
      </Link>

      {/* Play button - goes to play page without opening details */}
      <Link
        to="/movies/$id/play"
        params={{ id: String(id) }}
        className="absolute top-1/2 left-1/2 flex size-14 -translate-x-1/2 -translate-y-1/2 scale-90 items-center justify-center rounded-full bg-cyan-500 text-slate-900 opacity-0 shadow-lg shadow-black/30 transition-all duration-200 group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-cyan-400 focus:scale-100 focus:opacity-100 focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none"
        aria-label={`Play ${title}${year ? `, ${year}` : ""}`}
      >
        <Play className="size-7 fill-current" aria-hidden="true" />
      </Link>
    </article>
  );
}
