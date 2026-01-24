import { createLazyFileRoute } from "@tanstack/react-router";
import { Tv } from "lucide-react";
import ComingSoon from "@/components/ComingSoon";

export const Route = createLazyFileRoute("/_auth/tv-shows/")({
  component: TvShowsPage,
});

function TvShowsPage() {
  return (
    <>
      {/* React 19 Document Metadata */}
      <title>TV Shows - Igloo</title>
      <meta name="description" content="Browse and track your TV show library in your Igloo media center." />

      <ComingSoon
        title="TV Shows"
        description="Your TV show library is coming soon. Track episodes, discover new series, and never miss a premiere."
        icon={Tv}
      />
    </>
  );
}
