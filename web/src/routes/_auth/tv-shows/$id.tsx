import { createFileRoute } from "@tanstack/react-router";
import { Tv } from "lucide-react";
import ComingSoon from "@/components/shared/ComingSoon";

// No loader: there is no show detail endpoint yet, so this route must not fetch.
export const Route = createFileRoute("/_auth/tv-shows/$id")({
  component: TvShowDetailsPage,
});

function TvShowDetailsPage() {
  return (
    <>
      {/* React 19 Document Metadata */}
      <title>TV Show - Igloo</title>
      <meta name="description" content="TV show details are coming soon to your Igloo media center." />

      <ComingSoon
        title="TV Show Details"
        description="Episode lists, season artwork, and playback for this show are coming soon."
        icon={Tv}
      />
    </>
  );
}
