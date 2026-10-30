import {
  DETAIL_HERO_SCRIM_BOTTOM_CLASS,
  DETAIL_HERO_SCRIM_FADE_CLASS,
  DETAIL_HERO_SCRIM_SIDE_CLASS,
  MEDIA_FALLBACK_SURFACE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
type DetailBackdropProps = {
  backdropUrl: string | null;
};

export default function DetailBackdrop({
  backdropUrl,
}: DetailBackdropProps) {
  return (
    <div className="relative size-full" aria-hidden="true">
      {backdropUrl ? (
        <img
          src={backdropUrl}
          alt=""
          className="size-full object-cover"
        />
      ) : (
        <div className={cn("size-full", MEDIA_FALLBACK_SURFACE_CLASS)} />
      )}
      <div className={DETAIL_HERO_SCRIM_SIDE_CLASS} />
      <div className={DETAIL_HERO_SCRIM_BOTTOM_CLASS} />
      <div className={DETAIL_HERO_SCRIM_FADE_CLASS} />
    </div>
  );
}
