import { createLazyFileRoute } from "@tanstack/react-router";
import { Images } from "lucide-react";
import ComingSoon from "@/components/ComingSoon";

export const Route = createLazyFileRoute("/_auth/photos/")({
  component: PhotosPage,
});

function PhotosPage() {
  return (
    <>
      {/* React 19 Document Metadata */}
      <title>Photos - Igloo</title>
      <meta name="description" content="Browse and organize your personal photo gallery in your Igloo media center." />

      <ComingSoon
        title="Photos"
        description="Your personal photo gallery is coming soon. Organize, browse, and share your memories all in one place."
        icon={Images}
      />
    </>
  );
}
