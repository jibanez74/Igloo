import type { LucideIcon } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";

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
    <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
      {showImage ? (
        <img
          src={imageUrl}
          alt=""
          loading="lazy"
          decoding="async"
          fetchPriority="low"
          className="h-44 w-full object-cover object-center sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48"
          onError={onError}
        />
      ) : (
        <div className="flex h-44 w-full items-center justify-center bg-muted sm:h-52 md:aspect-21/9 md:min-h-48">
          <FallbackIcon className="size-16 text-muted-foreground opacity-40" />
        </div>
      )}
      <div className="absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent" />
    </div>
  );
}
