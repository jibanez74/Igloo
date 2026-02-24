import type { MovieDetailsGridItemView } from "@/lib/movie-details-view";

type MovieDetailsGridProps = {
  items: MovieDetailsGridItemView[];
  headingId?: string;
};

export default function MovieDetailsGrid({
  items,
  headingId = "details-heading",
}: MovieDetailsGridProps) {
  if (!items.length) return null;

  return (
    <section
      className="mt-10 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4 sm:p-6"
      aria-labelledby={headingId}
    >
      <h2
        id={headingId}
        tabIndex={-1}
        className="mb-4 text-lg font-semibold text-white outline-none"
      >
        Additional Details
      </h2>
      <dl className="grid grid-cols-2 gap-6 sm:grid-cols-3 lg:grid-cols-4">
        {items.map((item, index) => (
          <div
            key={`${item.label}-${index}`}
            tabIndex={0}
            className="-m-2 rounded-lg p-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50"
            role="group"
            aria-label={item.ariaLabel ?? `${item.label}: ${item.value}`}
          >
            <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
              {item.label}
            </dt>
            <dd className="mt-1 text-white">{item.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
