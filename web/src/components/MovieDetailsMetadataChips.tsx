import { Star, Clock, Calendar, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { OVER_MEDIA_BADGE_CLASS } from "@/lib/constants";
import { formatDate, formatSpokenRuntimeMinutes } from "@/lib/format";
import { audienceRatingClass, criticRatingClass } from "@/lib/rating";
import type { MovieDetailsMetadataChipsProps } from "@/types";

export default function MovieDetailsMetadataChips({
  criticRating,
  audienceRating,
  certificationLabel,
  runtime,
  runTimeMins,
  releaseDateStr,
  tmdbVoteAverage,
  capabilityBadges,
}: MovieDetailsMetadataChipsProps) {
  const spokenRuntime = formatSpokenRuntimeMinutes(runTimeMins);

  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
      aria-label="Movie details"
    >
      {tmdbVoteAverage != null && tmdbVoteAverage > 0 && (
        <li
          aria-label={`TMDB user score: ${tmdbVoteAverage.toFixed(1)} out of 10`}
        >
          <Badge variant="outline" className={OVER_MEDIA_BADGE_CLASS}>
            <span className="text-white/70">TMDB</span>
            <span aria-hidden="true" className="font-semibold">
              {tmdbVoteAverage.toFixed(1)}
            </span>
          </Badge>
        </li>
      )}
      {criticRating != null && criticRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${criticRatingClass(criticRating)}`}
          aria-label={`Critic rating: ${criticRating.toFixed(1)} out of 10`}
        >
          <Star className="size-3.5 fill-current" aria-hidden="true" />
          <span aria-hidden="true">{criticRating.toFixed(1)}</span>
        </li>
      )}
      {audienceRating != null && audienceRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${audienceRatingClass(audienceRating)}`}
          aria-label={`Audience rating: ${audienceRating.toFixed(1)} out of 10`}
        >
          <Users className="size-3.5 fill-current" aria-hidden="true" />
          <span aria-hidden="true">{audienceRating.toFixed(1)}</span>
        </li>
      )}
      {certificationLabel && (
        <li>
          <Badge
            variant="outline"
            className={`${OVER_MEDIA_BADGE_CLASS} font-semibold`}
          >
            {certificationLabel}
          </Badge>
        </li>
      )}
      {capabilityBadges?.map(badge => (
        <li key={badge.label} aria-label={badge.description}>
          <Badge
            variant="outline"
            className={`${OVER_MEDIA_BADGE_CLASS} font-semibold`}
          >
            <span aria-hidden="true">{badge.label}</span>
          </Badge>
        </li>
      ))}
      {runtime && (
        <li className="flex items-center gap-1.5 text-white/80">
          <Clock className="size-4" aria-hidden="true" />
          <time
            dateTime={runTimeMins != null ? `PT${runTimeMins}M` : undefined}
            aria-label={`Runtime: ${spokenRuntime ?? runtime}`}
          >
            <span aria-hidden="true">{runtime}</span>
          </time>
        </li>
      )}
      {releaseDateStr && (
        <li className="flex items-center gap-1.5 text-white/80">
          <Calendar className="size-4" aria-hidden="true" />
          <time dateTime={releaseDateStr}>{formatDate(releaseDateStr)}</time>
        </li>
      )}
    </ul>
  );
}
