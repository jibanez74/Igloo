import { Star, Clock, Calendar, Users } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { OVER_MEDIA_BADGE_CLASS } from "@/lib/constants";
import { formatDate, formatSpokenRuntimeMinutes } from "@/lib/format";
import { audienceRatingClass, criticRatingClass } from "@/lib/rating";
import type { MediaCapabilityBadge } from "@/types/movies";

type MovieDetailsMetadataChipsProps = {
  criticRating: number | null;
  audienceRating: number | null;
  certificationLabel: string | null;
  runtime: string | null;
  runTimeMins: number | null;
  releaseDateStr: string | null;
  tmdbVoteAverage?: number | null;
  capabilityBadges?: MediaCapabilityBadge[];
};

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
        <li>
          <Badge variant="outline" className={OVER_MEDIA_BADGE_CLASS}>
            <span className="sr-only">
              {`TMDB user score: ${tmdbVoteAverage.toFixed(1)} out of 10`}
            </span>
            <span aria-hidden="true" className="text-white/70">
              TMDB
            </span>
            <span aria-hidden="true" className="font-semibold">
              {tmdbVoteAverage.toFixed(1)}
            </span>
          </Badge>
        </li>
      )}
      {criticRating != null && criticRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${criticRatingClass(criticRating)}`}
        >
          <Star className="size-3.5 fill-current" aria-hidden="true" />
          <span className="sr-only">
            {`Critic rating: ${criticRating.toFixed(1)} out of 10`}
          </span>
          <span aria-hidden="true">{criticRating.toFixed(1)}</span>
        </li>
      )}
      {audienceRating != null && audienceRating > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${audienceRatingClass(audienceRating)}`}
        >
          <Users className="size-3.5 fill-current" aria-hidden="true" />
          <span className="sr-only">
            {`Audience rating: ${audienceRating.toFixed(1)} out of 10`}
          </span>
          <span aria-hidden="true">{audienceRating.toFixed(1)}</span>
        </li>
      )}
      {certificationLabel && (
        <li>
          <Badge
            variant="outline"
            className={`${OVER_MEDIA_BADGE_CLASS} font-semibold`}
          >
            <span className="sr-only">{`Rated ${certificationLabel}`}</span>
            <span aria-hidden="true">{certificationLabel}</span>
          </Badge>
        </li>
      )}
      {capabilityBadges?.map(badge => (
        <li key={badge.label}>
          <Badge
            variant="outline"
            className={`${OVER_MEDIA_BADGE_CLASS} font-semibold`}
          >
            <span className="sr-only">{badge.description}</span>
            <span aria-hidden="true">{badge.label}</span>
          </Badge>
        </li>
      ))}
      {runtime && (
        <li className="flex items-center gap-1.5 text-white/80">
          <Clock className="size-4" aria-hidden="true" />
          <time dateTime={runTimeMins != null ? `PT${runTimeMins}M` : undefined}>
            <span className="sr-only">
              {`Runtime: ${spokenRuntime ?? runtime}`}
            </span>
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
