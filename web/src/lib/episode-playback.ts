import { unwrapFloat } from "@/lib/nullable";
import { hasEligibleResumeProgress } from "@/lib/video-playback";
import type { ShowEpisodeType } from "@/types/shows";

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
