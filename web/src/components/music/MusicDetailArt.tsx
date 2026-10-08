import { Disc3, ListMusic, User } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import { cn } from "@/lib/utils";

type MusicDetailArtVariant = "album" | "musician" | "playlist";

// Shape, size and fallback per page. The skeleton reads the same entries, so
// its placeholder cannot drift from the art it stands in for (§3.4).
const ART_VARIANTS = {
  album: {
    figureClassName: "lg:pt-1",
    boxClassName: "w-44 rounded-xl sm:w-52 md:w-64 lg:w-72",
    fallbackClassName: "bg-muted",
    iconClassName: "text-muted-foreground",
    icon: Disc3,
    emptyLabel: "No cover available",
    alt: (name: string) => `Album cover for ${name}`,
  },
  musician: {
    figureClassName: "",
    boxClassName: "w-48 rounded-full md:w-56 lg:w-64",
    fallbackClassName: "bg-muted",
    iconClassName: "text-muted-foreground",
    icon: User,
    emptyLabel: "No image available",
    alt: (name: string) => `Photo of ${name}`,
  },
  // The gradient and tinted icon match the playlist cards' placeholder.
  playlist: {
    figureClassName: "",
    boxClassName: "w-40 rounded-xl sm:w-48 lg:w-56 xl:w-64",
    fallbackClassName: "bg-linear-to-br from-muted via-muted to-primary/30",
    iconClassName: "text-primary/20",
    icon: ListMusic,
    emptyLabel: "No cover available",
    alt: (name: string) => `Playlist cover for ${name}`,
  },
} as const;

const FIGURE_CLASS = "mx-auto shrink-0 lg:mx-0";

type MusicDetailArtProps = {
  variant: MusicDetailArtVariant;
  /** The cover or photo URL, or "" to show the fallback. */
  src: string;
  /** The album, musician or playlist name, for the alt text. */
  name: string;
};

/**
 * The square cover or round photo beside a music detail page's title. It is
 * the page's largest above-the-fold image, so like DetailHero's poster it
 * loads eagerly rather than with the lazy, low-priority card recipe (§3.3).
 */
export default function MusicDetailArt({
  variant,
  src,
  name,
}: MusicDetailArtProps) {
  const art = ART_VARIANTS[variant];
  const { showPoster: showImage, onError } = usePosterFallback(src);
  const FallbackIcon = art.icon;

  return (
    <figure className={cn(FIGURE_CLASS, art.figureClassName)}>
      <div
        className={cn(
          "aspect-square overflow-hidden border border-primary/20 bg-muted shadow-2xl shadow-primary/10",
          art.boxClassName,
        )}
      >
        {showImage ? (
          <img
            src={src}
            alt={art.alt(name)}
            width={640}
            height={640}
            decoding="async"
            className="size-full object-cover"
            onError={onError}
          />
        ) : (
          <div
            className={cn(
              "flex size-full items-center justify-center",
              art.fallbackClassName,
            )}
            role="img"
            aria-label={art.emptyLabel}
          >
            <FallbackIcon
              className={cn("size-16", art.iconClassName)}
              aria-hidden="true"
            />
          </div>
        )}
      </div>
    </figure>
  );
}

/** The art's placeholder for MusicDetailSkeleton: the same box, muted. */
export function MusicDetailArtSkeleton({
  variant,
}: {
  variant: MusicDetailArtVariant;
}) {
  const art = ART_VARIANTS[variant];

  return (
    <div className={cn(FIGURE_CLASS, art.figureClassName)}>
      <div className={cn("aspect-square bg-muted", art.boxClassName)} />
    </div>
  );
}
