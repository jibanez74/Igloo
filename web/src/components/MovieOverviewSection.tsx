import type { MovieOverviewSectionProps } from "@/types";

export default function MovieOverviewSection({
  overview,
}: MovieOverviewSectionProps) {
  return (
    <section className="mt-6 text-left" aria-labelledby="overview-heading">
      <h2
        id="overview-heading"
        tabIndex={-1}
        className="mb-2 text-lg font-semibold text-white outline-none sm:text-xl"
      >
        Overview
      </h2>
      <p className="text-[15px] leading-relaxed text-slate-300 sm:text-base">
        {overview || "No overview available."}
      </p>
    </section>
  );
}
