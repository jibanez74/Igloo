import { Skeleton } from "@/components/ui/skeleton";
import { MusicDetailArtSkeleton } from "@/components/music/MusicDetailArt";
import { MusicDetailBackdropSkeleton } from "@/components/music/MusicDetailBackdrop";
import SkeletonStatus from "@/components/shared/SkeletonStatus";
import {
  LIBRARY_POSTER_GRID_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MUSIC_DETAIL_ACTIONS_CLASS,
  MUSIC_DETAIL_HERO_ROW_CLASS,
  MUSIC_DETAIL_SHELL_CLASS,
} from "@/lib/constants";

type MusicDetailSkeletonProps = {
  /**
   * Which page it mirrors: the album and musician heroes over a backdrop
   * band, or the playlist header, which has no band and no overlap.
   */
  variant: "album" | "musician" | "playlist";
  /** The album page's admin "More options" button beside Play and Shuffle. */
  withMenu?: boolean;
};

// Authored beside the layouts it mirrors (design-system §3.4), from the same
// MUSIC_DETAIL_* rows and art sizes the pages use. The album and musician
// pages share one geometry: the backdrop band, the overlapping hero with its
// art box and text rows, then the track rows; only the art shape, the order of
// the pill rows and the musician's discography grid differ. The playlist page
// is a plain header - square cover beside the title - above its heading and
// rows, so it gets its own.
export default function MusicDetailSkeleton({
  variant,
  withMenu = false,
}: MusicDetailSkeletonProps) {
  const label = `Loading ${variant} details`;

  return (
    <SkeletonStatus label={label} className={MOTION_LOADING_STATE_CLASS}>
      {variant === "playlist" ? (
        <PlaylistSkeleton />
      ) : (
        <HeroSkeleton isMusician={variant === "musician"} withMenu={withMenu} />
      )}
    </SkeletonStatus>
  );
}

function HeroSkeleton({
  isMusician,
  withMenu,
}: {
  isMusician: boolean;
  withMenu: boolean;
}) {
  return (
    <>
      <MusicDetailBackdropSkeleton />

      <div className={MUSIC_DETAIL_SHELL_CLASS}>
        <div className={MUSIC_DETAIL_HERO_ROW_CLASS}>
          <MusicDetailArtSkeleton variant={isMusician ? "musician" : "album"} />

          <div className="min-w-0 flex-1 text-center lg:text-left">
            <Skeleton className="mx-auto h-10 max-w-lg lg:mx-0" />
            <Skeleton className="mx-auto mt-3 h-6 max-w-xs lg:mx-0" />
            {/* The musician page lists genres above its stats; the album
                page the other way round. */}
            {isMusician ? (
              <>
                <GenrePillsSkeleton />
                <StatPillsSkeleton />
              </>
            ) : (
              <>
                <StatPillsSkeleton />
                <GenrePillsSkeleton />
              </>
            )}
            <ActionsSkeleton withMenu={withMenu} />
          </div>
        </div>

        {/* The pages' lower stage: the hero row's mb-10 opens it, and its
            sections are spaced like the real space-y wrapper. */}
        <div className="space-y-8 sm:space-y-10">
          {isMusician && (
            <div>
              <Skeleton className="mb-4 h-7 w-40" />
              <div className={LIBRARY_POSTER_GRID_CLASS}>
                {Array.from({ length: 6 }).map((_, i) => (
                  <div
                    key={i}
                    className="overflow-hidden rounded-xl border border-border bg-card"
                  >
                    <Skeleton className="aspect-square rounded-none" />
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
            {isMusician && <Skeleton className="mb-4 h-7 w-32" />}
            <TrackRowsSkeleton />
          </div>
        </div>
      </div>
    </>
  );
}

// MusicStatList's three Badge pills.
function StatPillsSkeleton() {
  return (
    <div className="mt-4 flex flex-wrap justify-center gap-2 sm:gap-3 lg:justify-start">
      <Skeleton className="h-8 w-28 rounded-full" />
      <Skeleton className="h-8 w-24 rounded-full" />
      <Skeleton className="h-8 w-24 rounded-full" />
    </div>
  );
}

// MusicGenreList's pills.
function GenrePillsSkeleton() {
  return (
    <div className="mt-4 flex flex-wrap justify-center gap-2 lg:justify-start">
      <Skeleton className="h-7 w-20 rounded-full" />
      <Skeleton className="h-7 w-24 rounded-full" />
    </div>
  );
}

// PlayShuffleButtons' two lg (h-10) buttons, full width on phones, plus the
// album's icon-only admin menu button when the page shows one.
function ActionsSkeleton({ withMenu }: { withMenu: boolean }) {
  return (
    <div className={MUSIC_DETAIL_ACTIONS_CLASS}>
      <Skeleton className="h-10 w-full rounded-full sm:w-32" />
      <Skeleton className="h-10 w-full rounded-full sm:w-28" />
      {withMenu && (
        <Skeleton className="h-10 w-12 self-center rounded-full sm:self-auto" />
      )}
    </div>
  );
}

// The playlist header: cover, name, the two-figure stats row and the Play /
// Shuffle pair, then the "Tracks" rail heading over the rows.
function PlaylistSkeleton() {
  return (
    <>
      <div className={MUSIC_DETAIL_HERO_ROW_CLASS}>
        <MusicDetailArtSkeleton variant="playlist" />

        <div className="min-w-0 flex-1 text-center lg:text-left">
          <Skeleton className="mx-auto h-8 max-w-lg sm:h-9 md:h-10 lg:mx-0 lg:h-12" />
          <div className="mt-4 flex flex-wrap items-center justify-center gap-x-3 gap-y-2 sm:gap-x-4 lg:justify-start">
            <Skeleton className="h-4 w-16 sm:h-5 lg:h-6" />
            <Skeleton className="h-4 w-14 sm:h-5 lg:h-6" />
          </div>
          <ActionsSkeleton withMenu={false} />
        </div>
      </div>

      <Skeleton className="mb-4 h-7 w-24 sm:h-8" />
      <TrackRowsSkeleton />
    </>
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
