import { Calendar, CalendarRange, Film, Layers, Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { OVER_MEDIA_BADGE_CLASS } from "@/lib/constants";
import { formatDate } from "@/lib/format";
import { criticRatingClass } from "@/lib/rating";

type ShowDetailsMetadataChipsProps = {
  tmdbVoteAverage: number | null;
  certificationLabel: string | null;
  status: string | null;
  firstAirDate: string | null;
  lastAirDate: string | null;
  seasonCount: number;
  tmdbSeasonCount: number | null;
  availableEpisodeCount: number;
  tmdbEpisodeCount: number | null;
};

function pluralize(count: number, noun: string) {
  return `${count} ${count === 1 ? noun : `${noun}s`}`;
}

/**
 * Follows the established chip recipe (design-system §1.7): the spoken value
 * lives in an `sr-only` span and the formatted value is marked `aria-hidden`,
 * never an `aria-label` on the list item, whose implicit role does not support
 * one. Availability is stated in words, never by color alone.
 */
export default function ShowDetailsMetadataChips({
  tmdbVoteAverage,
  certificationLabel,
  status,
  firstAirDate,
  lastAirDate,
  seasonCount,
  tmdbSeasonCount,
  availableEpisodeCount,
  tmdbEpisodeCount,
}: ShowDetailsMetadataChipsProps) {
  // "3 of 5 seasons" only when TMDB knows of more than are here; otherwise the
  // bare count, so a fully present or unmatched show reads plainly.
  const seasonsPartial = tmdbSeasonCount != null && tmdbSeasonCount > seasonCount;
  const seasonsText = seasonsPartial
    ? `${seasonCount} of ${pluralize(tmdbSeasonCount, "season")}`
    : pluralize(seasonCount, "season");

  const episodesPartial =
    tmdbEpisodeCount != null && tmdbEpisodeCount > availableEpisodeCount;
  const episodesText = episodesPartial
    ? `${availableEpisodeCount} of ${pluralize(tmdbEpisodeCount, "episode")}`
    : pluralize(availableEpisodeCount, "episode");

  const firstYear = firstAirDate ? new Date(firstAirDate).getFullYear() : null;
  const lastYear = lastAirDate ? new Date(lastAirDate).getFullYear() : null;
  const airRange =
    firstYear != null
      ? lastYear != null && lastYear !== firstYear
        ? `${firstYear}–${lastYear}`
        : String(firstYear)
      : null;
  const spokenAirRange =
    firstYear != null
      ? lastYear != null && lastYear !== firstYear
        ? `Aired ${firstYear} to ${lastYear}`
        : `Aired ${firstYear}`
      : null;

  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
      aria-label="Show details"
    >
      {tmdbVoteAverage != null && tmdbVoteAverage > 0 && (
        <li
          className={`flex items-center gap-1.5 rounded-full px-3 py-1.5 font-bold ${criticRatingClass(tmdbVoteAverage)}`}
        >
          <Star className="size-3.5 fill-current" aria-hidden="true" />
          <span className="sr-only">
            {`TMDB user score: ${tmdbVoteAverage.toFixed(1)} out of 10`}
          </span>
          <span aria-hidden="true">{tmdbVoteAverage.toFixed(1)}</span>
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
      {status && (
        <li>
          <Badge variant="outline" className={OVER_MEDIA_BADGE_CLASS}>
            <span className="sr-only">{`Series status: ${status}`}</span>
            <span aria-hidden="true">{status}</span>
          </Badge>
        </li>
      )}
      <li className="flex items-center gap-1.5 text-white/80">
        <Layers className="size-4" aria-hidden="true" />
        <span className="sr-only">
          {seasonsPartial
            ? `${seasonsText} available in this library`
            : `${seasonsText} in this library`}
        </span>
        <span aria-hidden="true">{seasonsText}</span>
      </li>
      <li className="flex items-center gap-1.5 text-white/80">
        <Film className="size-4" aria-hidden="true" />
        <span className="sr-only">
          {episodesPartial
            ? `${episodesText} available in this library`
            : `${episodesText} in this library`}
        </span>
        <span aria-hidden="true">{episodesText}</span>
      </li>
      {airRange && (
        <li className="flex items-center gap-1.5 text-white/80">
          <CalendarRange className="size-4" aria-hidden="true" />
          <span className="sr-only">{spokenAirRange}</span>
          <span aria-hidden="true">{airRange}</span>
        </li>
      )}
      {firstAirDate && !airRange && (
        <li className="flex items-center gap-1.5 text-white/80">
          <Calendar className="size-4" aria-hidden="true" />
          <time dateTime={firstAirDate}>{formatDate(firstAirDate)}</time>
        </li>
      )}
    </ul>
  );
}
