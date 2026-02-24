type MovieOverviewSectionProps = {
  overview: string | null;
  headingId?: string;
};

export default function MovieOverviewSection({
  overview,
  headingId = "overview-heading",
}: MovieOverviewSectionProps) {
  return (
    <section className="mt-6" aria-labelledby={headingId}>
      <h2
        id={headingId}
        tabIndex={-1}
        className="mb-2 text-lg font-semibold text-white outline-none sm:mb-3"
      >
        Overview
      </h2>
      <p className="leading-relaxed text-slate-300">
        {overview || "No overview available."}
      </p>
    </section>
  );
}
