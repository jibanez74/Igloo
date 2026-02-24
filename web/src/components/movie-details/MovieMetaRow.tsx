import { Calendar, Clock, Star } from "lucide-react";
import { getRatingColorClass } from "@/lib/movie-details-view";

type MovieMetaRowProps = {
  rating?: number | null;
  runtime?: string | null;
  runtimeMinutes?: number | null;
  releaseDate?: string | null;
};

export default function MovieMetaRow({
  rating,
  runtime,
  runtimeMinutes,
  releaseDate,
}: MovieMetaRowProps) {
  const hasRating = rating != null && rating > 0;
  const hasRuntime = !!runtime;
  const hasReleaseDate = !!releaseDate;
  if (!hasRating && !hasRuntime && !hasReleaseDate) return null;

  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center gap-3"
      aria-label="Movie details"
    >
      {hasRating && (
        <li
          className={`flex min-h-11 min-w-11 items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${getRatingColorClass(rating!)}`}
          aria-label={`User rating: ${rating!.toFixed(1)} out of 10`}
        >
          <Star className="size-3.5 fill-current" aria-hidden="true" />
          <span aria-hidden="true">{rating!.toFixed(1)}</span>
        </li>
      )}
      {hasRuntime && (
        <li className="flex min-h-11 items-center gap-1.5 text-slate-300">
          <Clock className="size-4 text-slate-400" aria-hidden="true" />
          <time
            dateTime={
              runtimeMinutes != null ? `PT${runtimeMinutes}M` : undefined
            }
            aria-label={`Duration: ${runtime}`}
          >
            {runtime}
          </time>
        </li>
      )}
      {hasReleaseDate && (
        <li className="flex min-h-11 items-center gap-1.5 text-slate-300">
          <Calendar className="size-4 text-slate-400" aria-hidden="true" />
          <time dateTime={releaseDate}>
            {new Date(releaseDate).toLocaleDateString("en-US", {
              year: "numeric",
              month: "short",
              day: "numeric",
            })}
          </time>
        </li>
      )}
    </ul>
  );
}
