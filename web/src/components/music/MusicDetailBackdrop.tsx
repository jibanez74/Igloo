import { usePosterFallback } from "@/hooks/usePosterFallback";
import { cn } from "@/lib/utils";

// The band's height, shared by the image, its fallback and the skeleton: a
// fixed height on phones, then a 21:9 ratio clamped to a share of the viewport.
const BAND_CLASS =
  "h-44 w-full sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48";
// Out to the page's edges and up to the app header: the negative margins
// cancel the shell's padding on those three sides.
const BAND_SHELL_CLASS = "relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8";
const BAND_SCRIM_CLASS =
  "absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent";

type MusicDetailBackdropProps = {
  /** The album cover or musician photo, or "" for a plain band. */
  imageUrl: string;
};

// The decorative 21:9 band behind an album or musician hero. It repeats the
// artwork the cover block already names, so the whole band is aria-hidden.
// With no artwork it is a plain band: the cover block beside the title
// already shows the "no cover" icon, and a second one here read as a glitch.
export default function MusicDetailBackdrop({
  imageUrl,
}: MusicDetailBackdropProps) {
  const { showPoster: showImage, onError } = usePosterFallback(imageUrl);

  return (
    <div
      data-slot="music-detail-band"
      className={BAND_SHELL_CLASS}
      aria-hidden="true"
    >
      {showImage ? (
        <img
          src={imageUrl}
          alt=""
          loading="lazy"
          decoding="async"
          fetchPriority="low"
          className={cn(BAND_CLASS, "object-cover object-center")}
          onError={onError}
        />
      ) : (
        <div className={cn(BAND_CLASS, "bg-muted")} />
      )}
      <div className={BAND_SCRIM_CLASS} />
    </div>
  );
}

/** The band's placeholder for MusicDetailSkeleton: same box, no artwork. */
export function MusicDetailBackdropSkeleton() {
  return (
    <div
      data-slot="music-detail-band"
      className={BAND_SHELL_CLASS}
      aria-hidden="true"
    >
      <div className={cn(BAND_CLASS, "bg-muted")} />
      <div className={BAND_SCRIM_CLASS} />
    </div>
  );
}
