import { Film } from "lucide-react";
import type { MovieDetailsPosterBlockProps } from "@/types";

export default function MovieDetailsPosterBlock({
  posterUrl,
  movieTitle,
}: MovieDetailsPosterBlockProps) {
  return (
    <figure className="mx-auto min-w-0 shrink-0 lg:mx-0 lg:pt-1">
      <div className="w-44 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 sm:w-52 md:w-64 lg:w-72">
        {posterUrl ? (
          <img
            src={posterUrl}
            alt={`Movie poster for ${movieTitle}`}
            className="block aspect-2/3 w-full rounded-xl object-cover"
          />
        ) : (
          <div
            className="flex aspect-2/3 w-full items-center justify-center bg-slate-800"
            role="img"
            aria-label="No poster available"
          >
            <Film className="size-12 text-slate-600" aria-hidden="true" />
          </div>
        )}
      </div>
    </figure>
  );
}
