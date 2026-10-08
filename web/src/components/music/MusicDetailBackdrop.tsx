import type { LucideIcon } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import { cn } from "@/lib/utils";

// The band's height, shared by the image, its fallback and the skeleton: a
// fixed height on phones, then a 21:9 ratio clamped to a share of the viewport.
const BAND_CLASS =
  "h-44 w-full sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48";
const BAND_SHELL_CLASS = "relative -mx-4 sm:-mx-6 lg:-mx-8";
const BAND_SCRIM_CLASS =
  "absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent";

type MusicDetailBackdropProps = {
  /** The album cover or musician photo, or "" to show the fallback icon. */
  imageUrl: string;
  fallbackIcon: LucideIcon;
};

// The decorative 21:9 band behind an album or musician hero. It repeats the
// artwork the cover block already names, so the whole band is aria-hidden.
export default function MusicDetailBackdrop({
  imageUrl,
  fallbackIcon: FallbackIcon,
}: MusicDetailBackdropProps) {
  const { showPoster: showImage, onError } = usePosterFallback(imageUrl);

  return (
    <div className={BAND_SHELL_CLASS} aria-hidden="true">
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
        <div
          className={cn(BAND_CLASS, "flex items-center justify-center bg-muted")}
        >
          <FallbackIcon className="size-16 text-muted-foreground opacity-40" />
        </div>
      )}
      <div className={BAND_SCRIM_CLASS} />
    </div>
  );
}

/** The band's placeholder for MusicDetailSkeleton: same box, no artwork. */
export function MusicDetailBackdropSkeleton() {
  return (
    <div className={BAND_SHELL_CLASS} aria-hidden="true">
      <div className={cn(BAND_CLASS, "bg-muted")} />
      <div className={BAND_SCRIM_CLASS} />
    </div>
  );
}
