import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Play } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";
import { seasonPlayTarget } from "@/lib/episode-playback";
import { episodeCode } from "@/lib/format";
import { showSeasonEpisodesQueryOpts } from "@/lib/query-opts";
import { cn } from "@/lib/utils";

type ShowDetailsHeroActionsProps = {
  showId: number;
  selectedSeason: number;
};

/**
 * The hero's one action: start the selected season where the viewer is. It
 * resumes the first partly watched episode, else plays the first unwatched
 * one, else the season's first episode. It reads the same season query the
 * episode list renders from, so it costs no extra request and agrees with the
 * rows below it. Nothing renders until the season is known to have episodes.
 */
export default function ShowDetailsHeroActions({
  showId,
  selectedSeason,
}: ShowDetailsHeroActionsProps) {
  const { data } = useQuery(
    showSeasonEpisodesQueryOpts(showId, selectedSeason),
  );
  const episodes = data && !data.error ? data.data.episodes : [];
  const target = seasonPlayTarget(episodes);
  if (!target) return null;

  const code = episodeCode(selectedSeason, target.episode.episode_number);
  const verb = target.resume ? "Resume" : "Play";

  return (
    <div className="mt-6 flex flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start">
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
