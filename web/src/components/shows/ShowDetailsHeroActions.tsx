import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Play } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { seasonPlayTarget } from "@/lib/episode-playback";
import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants";
import { episodeCode } from "@/lib/format";
import { showSeasonEpisodesQueryOpts } from "@/lib/query-opts";
import { cn } from "@/lib/utils";

const ACTIONS_ROW_CLASS =
  "mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start";

type ShowDetailsHeroActionsProps = {
  showId: number;
  selectedSeason: number;
};

/**
 * The hero's one action: start the selected season where the viewer is. It
 * resumes the first partly watched episode, else plays the first unwatched
 * one, else the season's first episode. It reads the same season query the
 * episode list renders from, so it costs no extra request and agrees with the
 * rows below it. While the season loads (a season switch does not wait in the
 * route loader) it holds the button's place, so the hero does not grow when
 * the episodes land; the episode list announces the load. A season with no
 * episodes renders nothing.
 */
export default function ShowDetailsHeroActions({
  showId,
  selectedSeason,
}: ShowDetailsHeroActionsProps) {
  const { data, isPending } = useQuery(
    showSeasonEpisodesQueryOpts(showId, selectedSeason),
  );
  if (isPending) {
    return (
      <div className={ACTIONS_ROW_CLASS} aria-hidden="true">
        <div
          className={cn(
            "h-11 flex-1 rounded-md bg-muted sm:w-36 sm:flex-none",
            MOTION_LOADING_STATE_CLASS,
          )}
        />
      </div>
    );
  }

  const episodes = data && !data.error ? data.data.episodes : [];
  const target = seasonPlayTarget(episodes);
  if (!target) return null;

  const code = episodeCode(selectedSeason, target.episode.episode_number);
  const verb = target.resume ? "Resume" : "Play";

  return (
    <div className={ACTIONS_ROW_CLASS}>
      <Link
        to="/tv-shows/$id/episodes/$episodeId/play"
        params={{ id: String(showId), episodeId: String(target.episode.id) }}
        search={{ start: 0, audio_track: 0 }}
        className={cn(
          buttonVariants({ variant: "accent", size: "lg" }),
          "min-h-11 flex-1 touch-manipulation sm:flex-none",
        )}
        aria-label={`${verb} ${code} ${target.episode.name}`}
      >
        <Play className="size-4 fill-current" aria-hidden="true" />
        {verb} {code}
      </Link>
    </div>
  );
}
