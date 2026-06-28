import { formatCurrency } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { MovieAdditionalDetailsSectionProps } from "@/types";

export default function MovieAdditionalDetailsSection({
  status,
  language,
  budget,
  revenue,
}: MovieAdditionalDetailsSectionProps) {
  const showStatus = status != null && status.trim() !== "";

  return (
    <section
      className="mt-8 rounded-xl border border-amber-500/10 bg-slate-800/30 p-4 sm:mt-10 sm:p-5"
      aria-labelledby="details-heading"
    >
      <h2
        id="details-heading"
        tabIndex={-1}
        className="mb-4 text-xl font-semibold text-white outline-none sm:text-2xl"
      >
        Additional Details
      </h2>
      <dl
        className={cn(
          "grid grid-cols-1 gap-5 sm:grid-cols-2 sm:gap-6",
          showStatus ? "lg:grid-cols-4" : "lg:grid-cols-3",
        )}
      >
        {showStatus && (
          <div className="rounded-lg">
            <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
              Status
            </dt>
            <dd className="mt-1 text-white">{status.trim()}</dd>
          </div>
        )}
        <div className="rounded-lg">
          <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
            Original Language
          </dt>
          <dd className="mt-1 text-white uppercase">{language ?? "-"}</dd>
        </div>
        <div className="rounded-lg">
          <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
            Budget
          </dt>
          <dd className="mt-1 text-white">
            <data value={budget ?? 0}>{formatCurrency(budget ?? 0)}</data>
          </dd>
        </div>
        <div className="rounded-lg">
          <dt className="text-sm font-semibold tracking-wide text-amber-300/70 uppercase">
            Revenue
          </dt>
          <dd className="mt-1 text-white">
            <data value={revenue ?? 0}>{formatCurrency(revenue ?? 0)}</data>
          </dd>
        </div>
      </dl>
    </section>
  );
}
