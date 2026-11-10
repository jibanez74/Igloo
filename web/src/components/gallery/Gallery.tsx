import GalleryOverlayStage from "@/components/gallery/GalleryOverlays";
import GalleryPrimitives from "@/components/gallery/GalleryPrimitives";

type GalleryProps = {
  /** Name of one overlay to show open, alone on the page (`?open=dialog`). */
  open?: string;
};

/**
 * Every primitive in every variant and state, for review by eye and for the
 * screenshot baselines (e2e/visual). Development only: the route imports this
 * module behind `import.meta.env.DEV`, and CI greps the production bundle for
 * the `igloo-ui-gallery` marker below to prove it stayed out.
 */
export default function Gallery({ open }: GalleryProps) {
  return (
    <main
      data-gallery-root="igloo-ui-gallery"
      className="min-h-svh bg-card text-foreground"
    >
      {open ? (
        <GalleryOverlayStage name={open} />
      ) : (
        <div className="mx-auto flex max-w-5xl flex-col gap-6 p-6">
          <h1 className="text-2xl font-semibold tracking-tight text-foreground">
            Component gallery
          </h1>
          <GalleryPrimitives />
        </div>
      )}
    </main>
  );
}
