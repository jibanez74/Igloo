export default function MovieDetailsSkeleton() {
  return (
    <div
      className="animate-pulse"
      role="status"
      aria-label="Loading movie details"
    >
      <span className="sr-only">Loading movie details...</span>

      {/* Backdrop skeleton */}
      <div
        className="relative -mx-4 sm:-mx-6 lg:-mx-8 xl:-mx-12"
        aria-hidden="true"
      >
        <div className="aspect-21/9 bg-slate-800" />
        <div className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent" />
      </div>

      {/* Content skeleton */}
      <div className="relative z-10 -mt-32" aria-hidden="true">
        <div className="flex flex-col gap-6 md:flex-row lg:gap-8">
          {/* Poster skeleton */}
          <div className="mx-auto shrink-0 md:mx-0">
            <div className="aspect-2/3 w-48 rounded-xl bg-slate-800 md:w-64 lg:w-72" />
          </div>

          {/* Info skeleton */}
          <div className="flex-1 space-y-4">
            <div className="h-10 w-3/4 rounded-sm bg-slate-800" />
            <div className="h-5 w-1/2 rounded-sm bg-slate-800" />
            <div className="flex gap-2">
              <div className="h-6 w-20 rounded-full bg-slate-800" />
              <div className="h-6 w-24 rounded-full bg-slate-800" />
              <div className="h-6 w-16 rounded-full bg-slate-800" />
            </div>
            <div className="space-y-2 pt-4">
              <div className="h-4 w-full rounded-sm bg-slate-800" />
              <div className="h-4 w-full rounded-sm bg-slate-800" />
              <div className="h-4 w-3/4 rounded-sm bg-slate-800" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
