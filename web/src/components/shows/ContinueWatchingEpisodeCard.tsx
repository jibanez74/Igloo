import { useQueryClient } from "@tanstack/react-query";
import { Tv } from "lucide-react";
import PosterCard from "@/components/shared/PosterCard";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { episodeCode } from "@/lib/format";
import { showDetailsQueryOpts } from "@/lib/query-opts";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { ContinueWatchingEpisodeItemType } from "@/types";

type ContinueWatchingEpisodeCardProps = {
  episode: ContinueWatchingEpisodeItemType;
};

/**
 * An in-progress episode in the home "Continue Watching" row. It wears the
 * show's poster so the row keeps one aspect ratio, and names the episode in the
 * subtitle. Play goes straight to the episode; where it starts is the player's
 * resume dialog to offer, exactly as it is for a movie.
 */
export default function ContinueWatchingEpisodeCard({
  episode,
}: ContinueWatchingEpisodeCardProps) {
  const queryClient = useQueryClient();

  const handlePrefetch = () =>
    queryClient.prefetchQuery(showDetailsQueryOpts(episode.show_id));

  const code = episodeCode(episode.season_number, episode.episode_number);
  const subtitle = `${code} · ${episode.episode_name}`;
  const progressPct = Math.min(
    100,
    Math.max(
      0,
      Math.round((episode.progress_sec / episode.duration_sec) * 100),
    ),
  );

  const posterUrl =
    episode.poster_path.Valid && episode.poster_path.String !== ""
      ? buildTmdbImageUrl(episode.poster_path.String, TMDB_POSTER_SIZE)
      : "";

  return (
    <PosterCard
      detailsLink={{
        to: "/tv-shows/$id",
        params: { id: String(episode.show_id) },
      }}
      playLink={{
        to: "/tv-shows/$id/episodes/$episodeId/play",
        params: {
          id: String(episode.show_id),
          episodeId: String(episode.id),
        },
        search: { start: 0, audio_track: 0 },
      }}
      posterUrl={posterUrl}
      fallbackIcon={Tv}
      title={episode.title}
      subtitle={subtitle}
      detailsLabel={`${episode.title}, ${subtitle}, ${progressPct}% watched`}
      playLabel={`Resume ${episode.title} ${subtitle}`}
      watchProgress={{
        progressSec: episode.progress_sec,
        durationSec: episode.duration_sec,
      }}
      onPrefetch={handlePrefetch}
    />
  );
}
