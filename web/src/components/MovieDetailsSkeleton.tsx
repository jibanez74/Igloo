export default function MovieDetailsSkeleton() {
  return (
    <div
      className="animate-pulse"
      role="status"
      aria-label="Loading movie details"
    >
      <span className="sr-only">Loading movie details...</span>

      <div className="relative -mx-4 sm:-mx-6 lg:-mx-8" aria-hidden="true">
        <div className="h-44 w-full bg-slate-800 sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48" />
        <div className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent" />
      </div>

      <div
        className="relative z-10 -mt-20 sm:-mt-24 md:-mt-28 lg:-mt-32"
        aria-hidden="true"
      >
        <div className="flex min-w-0 flex-col gap-6 sm:gap-8 lg:flex-row lg:items-start lg:gap-10">
          <div className="mx-auto shrink-0 lg:mx-0 lg:pt-1">
            <div className="aspect-2/3 w-44 rounded-xl bg-slate-800 sm:w-52 md:w-64 lg:w-72" />
          </div>

          <div className="min-w-0 flex-1 space-y-4">
            <div className="mx-auto h-10 max-w-md rounded-sm bg-slate-800 lg:mx-0" />
            <div className="mx-auto h-5 max-w-xs rounded-sm bg-slate-800 lg:mx-0" />
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className="h-8 w-24 rounded-full bg-slate-800" />
              <div className="h-8 w-20 rounded-full bg-slate-800" />
              <div className="h-8 w-28 rounded-full bg-slate-800" />
            </div>
            <div className="space-y-2 pt-4 text-left">
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
