import { lazy, Suspense } from "react";
import { createFileRoute, redirect } from "@tanstack/react-router";
import { routeHead } from "@/lib/route-head";

// The component gallery exists in development only. Vite replaces
// `import.meta.env.DEV` with a literal, so a production build drops the
// import and the gallery's modules with it; the route itself stays so the
// generated route tree is the same in every mode, and sends visitors home.
const Gallery = import.meta.env.DEV
  ? lazy(() => import("@/components/gallery/Gallery"))
  : null;

const GALLERY_HEAD = routeHead("Component gallery");

type GallerySearch = {
  open?: string;
  section?: string;
};

export const Route = createFileRoute("/gallery")({
  head: () => GALLERY_HEAD,
  validateSearch: (search: Record<string, unknown>): GallerySearch => ({
    open: typeof search.open === "string" ? search.open : undefined,
    section: typeof search.section === "string" ? search.section : undefined,
  }),
  beforeLoad: () => {
    if (!Gallery) throw redirect({ to: "/" });
  },
  component: GalleryRoute,
});

function GalleryRoute() {
  const { open, section } = Route.useSearch();

  if (!Gallery) return null;

  return (
    <Suspense fallback={null}>
      <Gallery open={open} section={section} />
    </Suspense>
  );
}
