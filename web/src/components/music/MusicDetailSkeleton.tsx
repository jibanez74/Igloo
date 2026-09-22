import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants";

type MusicDetailSkeletonProps = {
  /**
   * Which hero it mirrors: the square cover an album and a playlist share, or
   * the musician's round thumb above a discography grid.
   */
  variant: "album" | "musician" | "playlist";
};

// Authored beside the layouts it mirrors (design-system §3.4): the backdrop
// band, the overlapping hero with its art box and text rows, then the track
// rows. The two music detail pages share this geometry; only the art shape,
// the hero's placeholder rows and the musician's discography grid differ.
export default function MusicDetailSkeleton({
  variant,
}: MusicDetailSkeletonProps) {
  const label = `Loading ${variant} details`;
  const hasSquareCover = variant !== "musician";

  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label={label}
    >
      <span className="sr-only">{label}...</span>

      <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
        <div className="h-44 w-full bg-muted sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48" />
        <div className="absolute inset-0 bg-linear-to-t from-background via-background/60 to-transparent" />
      </div>

      <div
        className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32"
        aria-hidden="true"
      >
        <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
          {hasSquareCover ? (
            <div className="mx-auto shrink-0 lg:mx-0 lg:pt-1">
              <div className="aspect-square w-44 rounded-xl bg-muted sm:w-52 md:w-64 lg:w-72" />
            </div>
          ) : (
            <div className="mx-auto shrink-0 lg:mx-0">
              <div className="aspect-square w-48 rounded-full bg-muted md:w-56 lg:w-64" />
            </div>
          )}

          <div className="min-w-0 flex-1 space-y-4 text-center lg:text-left">
            <div className="mx-auto h-10 max-w-lg rounded-sm bg-muted lg:mx-0" />
            <div className="mx-auto h-6 max-w-xs rounded-sm bg-muted lg:mx-0" />
            {hasSquareCover ? (
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
            ) : (
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
            )}
          </div>
        </div>

        {!hasSquareCover && (
          <div className="mt-10">
            <div className="mb-4 h-7 w-40 rounded-sm bg-muted" />
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
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

        <div className="mt-10">
          {!hasSquareCover && <div className="mb-4 h-7 w-32 rounded-sm bg-muted" />}
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
        </div>
      </div>
    </div>
  );
}
