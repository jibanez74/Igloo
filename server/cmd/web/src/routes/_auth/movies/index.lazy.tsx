import { createLazyFileRoute } from "@tanstack/react-router";
import { Film } from "lucide-react";
import ComingSoon from "@/components/ComingSoon";

export const Route = createLazyFileRoute("/_auth/movies/")({
  component: MoviesPage,
});

function MoviesPage() {
  return (
    <>
      {/* React 19 Document Metadata */}
      <title>Movies - Igloo</title>
      <meta name="description" content="Browse and organize your personal movie collection in your Igloo media library." />

      <ComingSoon
        title="Movies"
        description="Your personal movie library is coming soon. Browse, organize, and enjoy your film collection all in one place."
        icon={Film}
      />
    </>
  );
}
