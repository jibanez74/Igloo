import {
  DETAIL_HERO_SCRIM_BOTTOM_CLASS,
  DETAIL_HERO_SCRIM_FADE_CLASS,
  DETAIL_HERO_SCRIM_SIDE_CLASS,
} from "@/lib/constants";
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
        <div className="size-full bg-muted" />
      )}
      <div className={DETAIL_HERO_SCRIM_SIDE_CLASS} />
      <div className={DETAIL_HERO_SCRIM_BOTTOM_CLASS} />
      <div className={DETAIL_HERO_SCRIM_FADE_CLASS} />
    </div>
  );
}
