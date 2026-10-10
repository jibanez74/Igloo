import { Clock, Calendar } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import TmdbScoreBadge from "@/components/shared/TmdbScoreBadge";
import { OVER_MEDIA_BADGE_CLASS } from "@/lib/constants";
import { formatDate, formatSpokenRuntimeMinutes } from "@/lib/format";
import type { MediaCapabilityBadge } from "@/types/movies";

type MovieDetailsMetadataChipsProps = {
  certificationLabel: string | null;
  runtime: string | null;
  runTimeMins: number | null;
  releaseDateStr: string | null;
  tmdbVoteAverage?: number | null;
  capabilityBadges?: MediaCapabilityBadge[];
};

export default function MovieDetailsMetadataChips({
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
          <TmdbScoreBadge score={tmdbVoteAverage} />
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
