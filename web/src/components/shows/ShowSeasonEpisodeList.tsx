import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Check, Play, Tv } from "lucide-react";
import EmptyState from "@/components/shared/EmptyState";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import WatchProgressBar from "@/components/shared/WatchProgressBar";
import EpisodeWatchedToggle from "@/components/shows/EpisodeWatchedToggle";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import {
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  MOTION_LOADING_STATE_CLASS,
  TMDB_STILL_SIZE,
} from "@/lib/constants";
import { episodeResumeProgress } from "@/lib/episode-playback";
import {
  episodeCode,
  formatDate,
  formatMinutesLeft,
  formatRuntimeMinutes,
  pluralize,
  seasonLabel,
} from "@/lib/format";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import { showSeasonEpisodesQueryOpts } from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { ShowEpisodeType } from "@/types";

type ShowSeasonEpisodeListProps = {
  showId: number;
  seasonNumber: number;
};

const LIST_CLASS = cn(
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  "list-none divide-y divide-border/60",
);
const ROW_CLASS = "flex gap-3 p-3 sm:gap-4 sm:p-4";
const STILL_CLASS =
  "relative aspect-video w-32 shrink-0 overflow-hidden rounded-md bg-muted sm:w-40";

type EpisodeRowProps = {
  showId: number;
  seasonNumber: number;
  episode: ShowEpisodeType;
};

/**
 * One episode: still, title, metadata, and the two actions the row offers —
 * Play (a link to the episode player) and the watched toggle. Resume state
 * shows as a progress strip over the still plus a "N min left" note; a
 * watched episode shows a chip instead. Everything is keyed off the season
 * payload, which carries the viewer's own progress per episode.
 */
function EpisodeRow({ showId, seasonNumber, episode }: EpisodeRowProps) {
  const stillPath = unwrapString(episode.still_path);
  const stillUrl = buildTmdbImageUrl(stillPath, TMDB_STILL_SIZE);
  const overview = unwrapString(episode.overview);
  const airDate = unwrapString(episode.air_date);
  const runtime = formatRuntimeMinutes(unwrapInt(episode.tmdb_runtime));
  const headingId = `episode-${episode.id}-title`;
  const code = episodeCode(seasonNumber, episode.episode_number);
  const resume = episodeResumeProgress(episode);
  const metaParts: { key: string; node: ReactNode }[] = [];
  if (runtime) metaParts.push({ key: "runtime", node: runtime });
  if (airDate) {
    metaParts.push({
      key: "air-date",
      node: <time dateTime={airDate}>{formatDate(airDate)}</time>,
    });
  }
  if (resume) {
    metaParts.push({
      key: "left",
      node: formatMinutesLeft(resume.progressSec, resume.durationSec),
    });
  }

  return (
    <article className={ROW_CLASS} aria-labelledby={headingId}>
      <div className={STILL_CLASS}>
        {stillUrl !== "" ? (
          <img
            src={stillUrl}
            alt=""
            loading="lazy"
            decoding="async"
            className="size-full object-cover"
          />
        ) : (
          <div className="flex size-full items-center justify-center">
            <Tv className="size-5 text-muted-foreground" aria-hidden="true" />
          </div>
        )}
        {resume && (
          <WatchProgressBar
            progressSec={resume.progressSec}
            durationSec={resume.durationSec}
            trackClassName="bg-black/40"
            className="absolute inset-x-0 bottom-0 rounded-none"
          />
        )}
      </div>

      <div className="min-w-0 flex-1">
        {/* h4: the season heading above this list is the h3. */}
        <h4
          id={headingId}
          className="text-sm font-semibold wrap-break-word text-foreground sm:text-base"
        >
          <span className="text-muted-foreground">{episode.episode_number}.</span>{" "}
          {episode.name}
        </h4>

        {(metaParts.length > 0 || episode.watched) && (
          <p className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
            {metaParts.map((part, index) => (
              <span key={part.key} className="contents">
                {index > 0 && <span aria-hidden="true">·</span>}
                <span>{part.node}</span>
              </span>
            ))}
            {episode.watched && (
              <Badge variant="outline">
                <Check className="size-3 text-success" aria-hidden="true" />
                Watched
              </Badge>
            )}
          </p>
        )}

        {overview && (
          <p className="mt-1.5 line-clamp-2 text-sm/relaxed text-muted-foreground">
            {overview}
          </p>
        )}
      </div>

      <div className="flex shrink-0 flex-col items-end gap-1 sm:flex-row sm:items-start">
        <Link
          to="/tv-shows/$id/episodes/$episodeId/play"
          params={{ id: String(showId), episodeId: String(episode.id) }}
          search={{ start: 0, audio_track: 0 }}
          className={cn(
            buttonVariants({ variant: "accent", size: "icon" }),
            "min-h-10 min-w-10 touch-manipulation rounded-full",
          )}
          aria-label={`${resume ? "Resume" : "Play"} ${code} ${episode.name}`}
        >
          <Play className="size-4 fill-current" aria-hidden="true" />
        </Link>
        <EpisodeWatchedToggle
          showId={showId}
          seasonNumber={seasonNumber}
          episodeId={episode.id}
          watched={episode.watched}
          episodeCode={code}
        />
      </div>
    </article>
  );
}

/**
 * Three placeholder rows on the real row geometry, so arrival shifts nothing.
 * Also used by the page-level skeleton, which supplies its own status label.
 */
export function EpisodeRowsPlaceholder() {
  return (
    <ul className={LIST_CLASS} aria-hidden="true">
      {[0, 1, 2].map(row => (
        <li key={row} className={ROW_CLASS}>
          <div className={cn(STILL_CLASS, "bg-muted")} />
          <div className="min-w-0 flex-1 space-y-2">
            <div className="h-4 w-2/3 rounded-sm bg-muted" />
            <div className="h-3 w-1/3 rounded-sm bg-muted" />
            <div className="h-3 w-full rounded-sm bg-muted" />
          </div>
        </li>
      ))}
    </ul>
  );
}

/**
 * Owns its own loading, empty, and error states: its query is separate from the
 * page's, so switching seasons must not blank the whole page. Each resolution
 * is announced, since the tab change that triggers it says nothing by itself.
 */
export default function ShowSeasonEpisodeList({
  showId,
  seasonNumber,
}: ShowSeasonEpisodeListProps) {
  const { data, isPending, isError, refetch } = useQuery(
    showSeasonEpisodesQueryOpts(showId, seasonNumber),
  );

  const label = seasonLabel(seasonNumber);

  let body;
  let announcement = "";
  if (isPending) {
    body = (
      <div
        className={MOTION_LOADING_STATE_CLASS}
        role="status"
        aria-label="Loading episodes"
      >
        <span className="sr-only">Loading episodes...</span>
        <EpisodeRowsPlaceholder />
      </div>
    );
  } else if (isError || data.error) {
    const message =
      (data?.error ? data.message : null) ||
      "Failed to load episodes. Please try again.";
    // MoviesLoadError is a role="alert", so it announces itself; adding the
    // failure to LiveAnnouncer as well would announce it twice.
    body = (
      <MoviesLoadError
        message={message}
        onRetry={() => {
          refetch();
        }}
      />
    );
  } else {
    const episodes = data.data?.episodes ?? [];
    announcement =
      episodes.length === 0
        ? `${label}: no episodes in this library`
        : `${label}: ${pluralize(episodes.length, "episode")} in this library`;

    body =
      episodes.length === 0 ? (
        <EmptyState
          icon={Tv}
          title="No episodes yet"
          description={`No episodes of ${label} are in this library.`}
          bordered
        />
      ) : (
        <ul
          className={LIST_CLASS}
          aria-label={`${label} episodes, ${episodes.length} in this library`}
        >
          {episodes.map(episode => (
            <li key={episode.id}>
              <EpisodeRow
                showId={showId}
                seasonNumber={seasonNumber}
                episode={episode}
              />
            </li>
          ))}
        </ul>
      );
  }

  return (
    <section className="mt-4" aria-labelledby="episodes-heading">
      <h3 id="episodes-heading" className="sr-only">
        {label} episodes
      </h3>
      <LiveAnnouncer message={announcement} />
      {body}
    </section>
  );
}
