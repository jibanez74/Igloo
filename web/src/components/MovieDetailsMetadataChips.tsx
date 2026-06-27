import { Star, Clock, Calendar, Users } from "lucide-react";
import { formatDate } from "@/lib/format";
import type { MovieDetailsMetadataChipsProps } from "@/types";

function criticRatingColor(score: number) {
  if (score >= 7) return "bg-amber-500 text-slate-900";
  if (score >= 5) return "bg-amber-600 text-slate-900";
  return "bg-slate-600 text-white";
}

function audienceRatingColor(score: number) {
  if (score >= 7) return "bg-violet-600 text-white";
  if (score >= 5) return "bg-violet-700 text-white";
  return "bg-slate-600 text-white";
}

export default function MovieDetailsMetadataChips({
  criticRating,
  audienceRating,
  certificationLabel,
  runtime,
  runTimeMins,
  releaseDateStr,
  tmdbVoteAverage,
}: MovieDetailsMetadataChipsProps) {
  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
      aria-label="Movie details"
    >
      {tmdbVoteAverage != null && tmdbVoteAverage > 0 && (
        <li
          className="flex items-center gap-2 rounded-full border border-sky-400/35 bg-slate-800/90 px-3 py-1.5 text-sm font-semibold text-sky-100"
          aria-label={`TMDB user score: ${tmdbVoteAverage.toFixed(1)} out of 10`}
        >
          <span className="text-slate-400">TMDB</span>
          <span aria-hidden="true">{tmdbVoteAverage.toFixed(1)}</span>
        </li>
      )}
      {criticRating != null && criticRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${criticRatingColor(criticRating)}`}
          aria-label={`Critic rating: ${criticRating.toFixed(1)} out of 10`}
        >
          <Star className="size-3.5 fill-current" aria-hidden="true" />
          <span aria-hidden="true">{criticRating.toFixed(1)}</span>
        </li>
      )}
      {audienceRating != null && audienceRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${audienceRatingColor(audienceRating)}`}
          aria-label={`Audience rating: ${audienceRating.toFixed(1)} out of 10`}
        >
          <Users className="size-3.5 fill-current" aria-hidden="true" />
          <span aria-hidden="true">{audienceRating.toFixed(1)}</span>
        </li>
      )}
      {certificationLabel && (
        <li className="rounded-full border border-amber-500/35 bg-slate-800/90 px-3 py-1.5 text-sm font-semibold text-amber-200">
          {certificationLabel}
        </li>
      )}
      {runtime && (
        <li className="flex items-center gap-1.5 text-slate-300">
          <Clock className="size-4 text-slate-400" aria-hidden="true" />
          <time
            dateTime={runTimeMins != null ? `PT${runTimeMins}M` : undefined}
            aria-label={`Duration: ${runtime}`}
          >
            {runtime}
          </time>
        </li>
      )}
      {releaseDateStr && (
        <li className="flex items-center gap-1.5 text-slate-300">
          <Calendar className="size-4 text-slate-400" aria-hidden="true" />
          <time dateTime={releaseDateStr}>{formatDate(releaseDateStr)}</time>
        </li>
      )}
    </ul>
  );
}
