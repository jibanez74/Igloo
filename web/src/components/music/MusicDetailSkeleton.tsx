import { MusicDetailArtSkeleton } from "@/components/music/MusicDetailArt";
import { MusicDetailBackdropSkeleton } from "@/components/music/MusicDetailBackdrop";
import {
  LIBRARY_POSTER_GRID_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MUSIC_DETAIL_HERO_ROW_CLASS,
  MUSIC_DETAIL_SHELL_CLASS,
} from "@/lib/constants";

type MusicDetailSkeletonProps = {
  /**
   * Which page it mirrors: the album and musician heroes over a backdrop
   * band, or the playlist header, which has no band and no overlap.
   */
  variant: "album" | "musician" | "playlist";
};

// Authored beside the layouts it mirrors (design-system §3.4). The album and
// musician pages share one geometry: the backdrop band, the overlapping hero
// with its art box and text rows, then the track rows; only the art shape,
// the hero's placeholder rows and the musician's discography grid differ. The
// playlist page is a plain header - square cover beside the title, two pills -
// above its heading and rows, so it gets its own.
export default function MusicDetailSkeleton({
  variant,
}: MusicDetailSkeletonProps) {
  const label = `Loading ${variant} details`;

  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label={label}
    >
      <span className="sr-only">{label}...</span>

      {variant === "playlist" ? (
        <PlaylistSkeleton />
      ) : (
        <HeroSkeleton isMusician={variant === "musician"} />
      )}
    </div>
  );
}

function HeroSkeleton({ isMusician }: { isMusician: boolean }) {
  return (
    <>
      <MusicDetailBackdropSkeleton />

      <div className={MUSIC_DETAIL_SHELL_CLASS} aria-hidden="true">
        <div className={MUSIC_DETAIL_HERO_ROW_CLASS}>
          <MusicDetailArtSkeleton variant={isMusician ? "musician" : "album"} />

          <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
            <div className="mx-auto h-10 max-w-lg rounded-sm bg-muted lg:mx-0" />
            <div className="mx-auto h-6 max-w-xs rounded-sm bg-muted lg:mx-0" />
            {isMusician ? (
              <>
                <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
                  <div className="h-7 w-20 rounded-full bg-muted" />
                  <div className="h-7 w-24 rounded-full bg-muted" />
                </div>
                <div className="flex flex-wrap justify-center gap-4 lg:justify-start">
                  <div className="h-5 w-20 rounded-sm bg-muted" />
                  <div className="h-5 w-20 rounded-sm bg-muted" />
                  <div className="h-5 w-16 rounded-sm bg-muted" />
                </div>
                <div className="flex flex-col gap-3 sm:flex-row sm:justify-center lg:justify-start">
                  <div className="h-12 w-full rounded-full bg-muted sm:w-32" />
                  <div className="h-12 w-full rounded-full bg-muted sm:w-28" />
                </div>
              </>
            ) : (
              <>
                <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
                  <div className="h-8 w-28 rounded-full bg-muted" />
                  <div className="h-8 w-24 rounded-full bg-muted" />
                  <div className="h-8 w-24 rounded-full bg-muted" />
                </div>
                <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
                  <div className="h-7 w-20 rounded-full bg-muted" />
                  <div className="h-7 w-24 rounded-full bg-muted" />
                </div>
                <div className="flex flex-col gap-3 pt-2 sm:flex-row sm:flex-wrap sm:justify-center lg:justify-start">
                  <div className="h-12 w-full rounded-full bg-muted sm:w-32" />
                  <div className="h-12 w-full rounded-full bg-muted sm:w-24" />
                  <div className="mx-auto size-12 rounded-full bg-muted sm:mx-0" />
                </div>
              </>
            )}
          </div>
        </div>

        {/* The pages' lower stage: the hero row's mb-10 opens it, and its
            sections are spaced like the real space-y wrapper. */}
        <div className="space-y-8 sm:space-y-10">
          {isMusician && (
            <div>
              <div className="mb-4 h-7 w-40 rounded-sm bg-muted" />
              <div className={LIBRARY_POSTER_GRID_CLASS}>
                {Array.from({ length: 6 }).map((_, i) => (
                  <div
                    key={i}
                    className="overflow-hidden rounded-xl border border-border bg-card"
                  >
                    <div className="aspect-square bg-muted" />
                    <div className="space-y-2 p-3">
                      <div className="h-4 w-3/4 rounded-sm bg-accent" />
                      <div className="h-3 w-1/2 rounded-sm bg-accent" />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div>
            {isMusician && <div className="mb-4 h-7 w-32 rounded-sm bg-muted" />}
            <TrackRowsSkeleton />
          </div>
        </div>
      </div>
    </>
  );
}

// The playlist header: cover, name, the two-figure stats row and the Play /
// Shuffle pair, then the "Tracks" rail heading over the rows.
function PlaylistSkeleton() {
  return (
    <div aria-hidden="true">
      <div className={MUSIC_DETAIL_HERO_ROW_CLASS}>
        <MusicDetailArtSkeleton variant="playlist" />

        <div className="min-w-0 flex-1 text-center lg:text-left">
          <div className="mx-auto h-8 max-w-lg rounded-sm bg-muted sm:h-9 md:h-10 lg:mx-0 lg:h-12" />
          <div className="mt-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 sm:gap-x-4 lg:justify-start">
            <div className="h-4 w-16 rounded-sm bg-muted sm:h-5 lg:h-6" />
            <div className="h-4 w-14 rounded-sm bg-muted sm:h-5 lg:h-6" />
          </div>
          <div className="mt-5 flex flex-col justify-center gap-2 sm:mt-6 sm:flex-row sm:gap-3 lg:justify-start">
            <div className="h-12 w-full rounded-full bg-muted sm:w-32" />
            <div className="h-12 w-full rounded-full bg-muted sm:w-28" />
          </div>
        </div>
      </div>

      <div className="mb-4 h-7 w-24 rounded-sm bg-muted sm:h-8" />
      <TrackRowsSkeleton />
    </div>
  );
}

function TrackRowsSkeleton() {
  return (
    <div className="space-y-2">
      {Array.from({ length: 8 }).map((_, i) => (
        <div
          key={i}
          className="flex h-14 items-center gap-4 rounded-lg bg-muted/50"
        >
          <div className="ml-4 h-4 w-6 rounded-sm bg-accent" />
          <div className="h-4 max-w-xs flex-1 rounded-sm bg-accent" />
          <div className="mr-4 h-4 w-16 rounded-sm bg-accent" />
        </div>
      ))}
    </div>
  );
}
