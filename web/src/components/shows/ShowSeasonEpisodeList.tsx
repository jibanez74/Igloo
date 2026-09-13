import { useQuery } from "@tanstack/react-query";
import { Tv } from "lucide-react";
import EmptyState from "@/components/shared/EmptyState";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import {
  DETAIL_TRACK_LIST_CONTAINER_CLASS,
  MOTION_LOADING_STATE_CLASS,
  TMDB_STILL_SIZE,
} from "@/lib/constants";
import { formatDate, formatRuntimeMinutes, seasonLabel } from "@/lib/format";
import { unwrapInt, unwrapString } from "@/lib/nullable";
import { showSeasonEpisodesQueryOpts } from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { ShowEpisodeType } from "@/types";

type ShowSeasonEpisodeListProps = {
  showId: number;
  seasonNumber: number;
};

const ROW_CLASS = "flex gap-3 p-3 sm:gap-4 sm:p-4";
const STILL_CLASS = "aspect-video w-32 shrink-0 overflow-hidden rounded-md bg-muted sm:w-40";

function EpisodeRow({ episode }: { episode: ShowEpisodeType }) {
  const stillPath = unwrapString(episode.still_path);
  const stillUrl = buildTmdbImageUrl(stillPath, TMDB_STILL_SIZE);
  const overview = unwrapString(episode.overview);
  const airDate = unwrapString(episode.air_date);
  const runtime = formatRuntimeMinutes(unwrapInt(episode.tmdb_runtime));

  return (
    <article className={ROW_CLASS}>
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
      </div>

      <div className="min-w-0 flex-1">
        <h3 className="text-sm font-semibold text-foreground sm:text-base">
          <span className="text-muted-foreground">{episode.episode_number}.</span>{" "}
          {episode.name}
        </h3>

        {(runtime || airDate) && (
          <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
            {runtime && <span>{runtime}</span>}
            {runtime && airDate && <span aria-hidden="true">·</span>}
            {airDate && <time dateTime={airDate}>{formatDate(airDate)}</time>}
          </p>
        )}

        {overview && (
          <p className="mt-1.5 line-clamp-2 text-sm/relaxed text-muted-foreground">
            {overview}
          </p>
        )}
      </div>
    </article>
  );
}

/** Skeleton rows reuse the real row geometry, so arrival shifts nothing. */
function EpisodeListSkeleton() {
  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label="Loading episodes"
    >
      <span className="sr-only">Loading episodes...</span>
      <ul className={cn(DETAIL_TRACK_LIST_CONTAINER_CLASS, "list-none divide-y divide-border/60")}>
        {[0, 1, 2].map(row => (
          <li key={row} className={ROW_CLASS} aria-hidden="true">
            <div className={cn(STILL_CLASS, "bg-muted")} />
            <div className="min-w-0 flex-1 space-y-2">
              <div className="h-4 w-2/3 rounded-sm bg-muted" />
              <div className="h-3 w-1/3 rounded-sm bg-muted" />
              <div className="h-3 w-full rounded-sm bg-muted" />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Owns its own loading, empty, and error states: its query is separate from the
 * page's, so switching seasons must not blank the whole page.
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
  if (isPending) {
    body = <EpisodeListSkeleton />;
  } else if (isError || data.error) {
    body = (
      <MoviesLoadError
        message={
          (data && data.error ? data.message : null) ||
          "Failed to load episodes. Please try again."
        }
        onRetry={() => {
          refetch();
        }}
      />
    );
  } else {
    const episodes = data.data?.episodes ?? [];

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
          className={cn(
            DETAIL_TRACK_LIST_CONTAINER_CLASS,
            "list-none divide-y divide-border/60",
          )}
          aria-label={`${label} episodes, ${episodes.length} in this library`}
        >
          {episodes.map(episode => (
            <li key={episode.id}>
              <EpisodeRow episode={episode} />
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
      {body}
    </section>
  );
}
