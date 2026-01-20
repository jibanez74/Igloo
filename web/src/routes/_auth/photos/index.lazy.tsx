import { createLazyFileRoute } from "@tanstack/react-router";
import { Images } from "lucide-react";
import ComingSoon from "@/components/ComingSoon";

export const Route = createLazyFileRoute("/_auth/photos/")({
  component: PhotosPage,
});

function PhotosPage() {
  return (
    <ComingSoon
      title="Photos"
      description="Your personal photo gallery is coming soon. Organize, browse, and share your memories all in one place."
      icon={Images}
    />
  );
}
