import { useQuery } from "@tanstack/react-query";
import { latestShowsQueryOpts } from "@/lib/query-opts";
import { Tv } from "lucide-react";
import HomeMediaSection from "@/components/home/HomeMediaSection";
import ShowCard from "@/components/shows/ShowCard";
import { HOME_POSTER_GRID_CLASS } from "@/lib/constants";
import { apiErrorMessage } from "@/lib/is-api-failure";

export default function LatestShows() {
  const { data, isPending } = useQuery(latestShowsQueryOpts());

  const shows = data && !data.error ? (data.data?.shows ?? []) : [];
  const errorMessage = data?.error
    ? apiErrorMessage(data, "Failed to load shows. Please try again later.")
    : undefined;

  return (
    <HomeMediaSection
      title="Recently Added Shows"
      headingId="recent-shows"
      items={shows}
      isPending={isPending}
      errorMessage={errorMessage}
      loadingLabel="Loading shows..."
      emptyTitle="No Shows Yet"
      emptyDescription="Your TV show library is empty. Add a shows folder in settings and run a scan to get started."
      emptyIcon={Tv}
      countNoun="show"
      gridClassName={HOME_POSTER_GRID_CLASS}
      getKey={show => String(show.id)}
      renderItem={show => <ShowCard show={show} />}
    />
  );
}
