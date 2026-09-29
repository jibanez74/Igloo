import { createFileRoute } from "@tanstack/react-router";
import { Images } from "lucide-react";
import ComingSoon from "@/components/shared/ComingSoon";
import { routeHead } from "@/lib/route-head";

const PHOTOS_HEAD = routeHead(
  "Photos",
  "Browse and organize your personal photo gallery in your Igloo media center.",
);

export const Route = createFileRoute("/_auth/photos/")({
  head: () => PHOTOS_HEAD,
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
