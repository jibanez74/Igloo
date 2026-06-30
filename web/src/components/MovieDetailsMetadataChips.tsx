import { Star, Clock, Calendar, Users } from "lucide-react";
import { formatDate, formatSpokenRuntimeMinutes } from "@/lib/format";
import type { MovieDetailsMetadataChipsProps } from "@/types";

function criticRatingColor(score: number) {
  if (score >= 7) return "bg-aurora text-aurora-foreground";
  if (score >= 5) return "bg-aurora/90 text-aurora-foreground";
  return "bg-muted text-foreground";
}

function audienceRatingColor(score: number) {
  if (score >= 7) return "bg-primary text-primary-foreground";
  if (score >= 5) return "bg-accent text-accent-foreground";
  return "bg-muted text-foreground";
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
  const spokenRuntime = formatSpokenRuntimeMinutes(runTimeMins);

  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
      aria-label="Movie details"
    >
      {tmdbVoteAverage != null && tmdbVoteAverage > 0 && (
        <li
          className="flex items-center gap-2 rounded-full border border-primary/35 bg-muted/90 px-3 py-1.5 text-sm font-semibold text-primary"
          aria-label={`TMDB user score: ${tmdbVoteAverage.toFixed(1)} out of 10`}
        >
          <span className="text-muted-foreground">TMDB</span>
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
        <li className="rounded-full border border-primary/35 bg-muted/90 px-3 py-1.5 text-sm font-semibold text-primary">
          {certificationLabel}
        </li>
      )}
      {runtime && (
        <li className="flex items-center gap-1.5 text-muted-foreground">
          <Clock className="size-4 text-muted-foreground" aria-hidden="true" />
          <time
            dateTime={runTimeMins != null ? `PT${runTimeMins}M` : undefined}
            aria-label={`Runtime: ${spokenRuntime ?? runtime}`}
          >
            <span aria-hidden="true">{runtime}</span>
          </time>
        </li>
      )}
      {releaseDateStr && (
        <li className="flex items-center gap-1.5 text-muted-foreground">
          <Calendar className="size-4 text-muted-foreground" aria-hidden="true" />
          <time dateTime={releaseDateStr}>{formatDate(releaseDateStr)}</time>
        </li>
      )}
    </ul>
  );
}
