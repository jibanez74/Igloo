import { TMDB_STILL_SIZE } from "@/lib/constants";
import { episodeCode } from "@/lib/format";
import { unwrapFloat, unwrapString } from "@/lib/nullable";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { hasEligibleResumeProgress } from "@/lib/video-playback";
import type { ShowEpisodeType, ShowEpisodeUpNextType } from "@/types/shows";

export type EpisodeResumeProgress = {
  progressSec: number;
  durationSec: number;
};

/**
 * The saved position an episode row can resume from, or null when there is
 * nothing to resume: never watched far enough, already finished, or marked
 * watched. Same eligibility rule as the movie hero and the resume dialog.
 */
export function episodeResumeProgress(
  episode: Pick<ShowEpisodeType, "progress_sec" | "duration_sec" | "watched">,
): EpisodeResumeProgress | null {
  if (episode.watched) return null;

  const progressSec = unwrapFloat(episode.progress_sec);
  const durationSec = unwrapFloat(episode.duration_sec);
  if (
    progressSec === null ||
    durationSec === null ||
    !hasEligibleResumeProgress(progressSec, durationSec)
  ) {
    return null;
  }

  return { progressSec, durationSec };
}

export type EpisodeUpNextPresentation = {
  /** "S1 E4 · Episode name" */
  title: string;
  stillUrl: string | null;
  /** True when the next episode continues from a saved position. */
  resume: boolean;
  /** Whole seconds the hand-off should start the next episode at. */
  startSec: number;
};

/**
 * How the up-next card names the next episode and where the hand-off starts
 * it. Split from the route so the navigation stays the route's business and
 * this mapping can be read — and tested — on its own.
 */
export function episodeUpNextPresentation(
  episode: ShowEpisodeUpNextType,
): EpisodeUpNextPresentation {
  const resume = episodeResumeProgress(episode);

  return {
    title: `${episodeCode(episode.season_number, episode.episode_number)} · ${episode.name}`,
    stillUrl: buildTmdbImageUrl(
      unwrapString(episode.still_path),
      TMDB_STILL_SIZE,
    ),
    resume: resume !== null,
    startSec: Math.floor(resume?.progressSec ?? 0),
  };
}

export type SeasonPlayTarget = {
  episode: ShowEpisodeType;
  /** True when the target continues a partly watched episode. */
  resume: boolean;
};

/**
 * What the hero Play button starts for a season: the first episode with a
 * resumable position, else the first unwatched episode, else the first
 * episode. Null for an empty season.
 */
export function seasonPlayTarget(
  episodes: ShowEpisodeType[],
): SeasonPlayTarget | null {
  if (episodes.length === 0) return null;

  const inProgress = episodes.find(
    (episode) => episodeResumeProgress(episode) !== null,
  );
  if (inProgress) return { episode: inProgress, resume: true };

  const unwatched = episodes.find((episode) => !episode.watched);
  return { episode: unwatched ?? episodes[0], resume: false };
}
