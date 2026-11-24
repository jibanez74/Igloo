import { DETAIL_SECTION_HEADING_CLASS } from "@/lib/constants";

type OverviewSectionProps = {
  overview: string | null;
};

export default function OverviewSection({ overview }: OverviewSectionProps) {
  return (
    <section className="mt-6 text-left" aria-labelledby="overview-heading">
      <h2
        id="overview-heading"
        tabIndex={-1}
        className={DETAIL_SECTION_HEADING_CLASS}
      >
        Overview
      </h2>
      {/* eslint-disable-next-line shadcn/no-arbitrary-values -- a step between
          text-sm and text-base, for a phone-width overview only. */}
      <p className="text-[15px] leading-relaxed text-muted-foreground sm:text-base">
        {overview || "No overview available."}
      </p>
    </section>
  );
}
