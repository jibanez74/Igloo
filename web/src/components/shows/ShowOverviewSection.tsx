type ShowOverviewSectionProps = {
  overview: string | null;
};

export default function ShowOverviewSection({
  overview,
}: ShowOverviewSectionProps) {
  return (
    <section className="mt-6 text-left" aria-labelledby="overview-heading">
      <h2
        id="overview-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-foreground outline-hidden sm:text-xl"
      >
        Overview
      </h2>
      <p className="text-[15px] leading-relaxed text-muted-foreground sm:text-base">
        {overview || "No overview available."}
      </p>
    </section>
  );
}
