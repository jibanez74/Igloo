import { useQuery } from "@tanstack/react-query";
import { continueWatchingQueryOpts } from "@/lib/query-opts";
import { Play } from "lucide-react";
import HomeMediaSection from "@/components/home/HomeMediaSection";
import MovieCard from "@/components/movies/MovieCard";
import ContinueWatchingEpisodeCard from "@/components/shows/ContinueWatchingEpisodeCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";
import type { ContinueWatchingItemType } from "@/types";

const renderCard = (item: ContinueWatchingItemType) => {
  if (item.kind === "episode") {
    return <ContinueWatchingEpisodeCard episode={item} />;
  }

  return (
    <MovieCard
      movie={item}
      watchProgress={{
        progressSec: item.progress_sec,
        durationSec: item.duration_sec,
      }}
    />
  );
};

export default function ContinueWatching() {
  const { data, isPending } = useQuery(continueWatchingQueryOpts());

  const items = data && !data.error ? (data.data?.items ?? []) : [];
  const hasError = data && data.error;
  const errorMessage = hasError
    ? data.message || "Failed to load what you are watching."
    : undefined;

  // The home route loader awaits this query, so the section renders populated
  // or not at all — never a loading block that collapses (layout shift).
  if (isPending) return null;
  if (!errorMessage && items.length === 0) return null;

  return (
    <HomeMediaSection
      title="Continue Watching"
      headingId="continue-watching"
      items={items}
      errorMessage={errorMessage}
      emptyTitle="Nothing In Progress"
      emptyDescription="Movies and episodes you start watching will appear here."
      emptyIcon={Play}
      countNoun="title"
      gridClassName={HOME_POSTER_GRID_CLASS}
      // A movie and an episode can share an id, so the kind is part of the key.
      getKey={item => `${item.kind}-${item.id}`}
      renderItem={renderCard}
    />
  );
}
