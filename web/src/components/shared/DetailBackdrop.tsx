import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  DETAIL_HERO_SCRIM_BOTTOM_CLASS,
  DETAIL_HERO_SCRIM_FADE_CLASS,
  DETAIL_HERO_SCRIM_SIDE_CLASS,
  MEDIA_FALLBACK_SURFACE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type DetailBackdropProps = {
  backdropUrl: string | null;
  /** The title's poster, shown blurred over the tile when there is no backdrop. */
  posterUrl?: string | null;
};

export default function DetailBackdrop({
  backdropUrl,
  posterUrl = null,
}: DetailBackdropProps) {
  const { showPoster, onError } = usePosterFallback(posterUrl ?? "");

  return (
    <div className="relative size-full overflow-hidden" aria-hidden="true">
      {backdropUrl ? (
        <img
          src={backdropUrl}
          alt=""
          className="size-full object-cover"
        />
      ) : (
        <>
          {/* The fixed navy tile is always the base so the literal scrims
              never meet a themed surface (design-system §1.2). */}
          <div className={cn("absolute inset-0", MEDIA_FALLBACK_SURFACE_CLASS)} />
          {showPoster && (
            // A title with a poster but no backdrop still gets a media band:
            // the poster, blurred and scaled past the edges so the blur has
            // no visible border; the scrims below carry the title as usual.
            <img
              src={posterUrl ?? undefined}
              alt=""
              onError={onError}
              className="absolute inset-0 size-full scale-110 object-cover blur-2xl"
            />
          )}
        </>
      )}
      <div className={DETAIL_HERO_SCRIM_SIDE_CLASS} />
      <div className={DETAIL_HERO_SCRIM_BOTTOM_CLASS} />
      <div className={DETAIL_HERO_SCRIM_FADE_CLASS} />
    </div>
  );
}
