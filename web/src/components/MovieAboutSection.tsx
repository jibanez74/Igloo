import { formatCurrency } from "@/lib/format";
import type { MovieAboutSectionProps } from "@/types";

function AboutRow({ label, children }: { label: string; children: string }) {
  return (
    <div className="flex flex-wrap gap-x-2">
      <dt className="shrink-0 text-muted-foreground">{label}:</dt>
      <dd className="min-w-0 text-foreground">{children}</dd>
    </div>
  );
}

/**
 * Compact fine-print block merging production companies and catalog facts
 * (Netflix-style "About" footer). Replaces the old Additional Details and
 * Production Companies sections.
 */
export default function MovieAboutSection({
  movieTitle,
  status,
  language,
  budget,
  revenue,
  companies,
}: MovieAboutSectionProps) {
  const statusLabel = status != null && status.trim() !== "" ? status.trim() : null;
  const companyNames = companies.map(pc => pc.name).join(", ");

  return (
    <section
      className="mt-10 border-t border-border pt-6 sm:mt-12"
      aria-labelledby="details-heading"
    >
      <h2
        id="details-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-foreground outline-none sm:text-xl"
      >
        About {movieTitle}
      </h2>
      <dl className="max-w-3xl space-y-1.5 text-sm">
        {companyNames !== "" && (
          <AboutRow label="Production">{companyNames}</AboutRow>
        )}
        {statusLabel && <AboutRow label="Status">{statusLabel}</AboutRow>}
        {language != null && language.trim() !== "" && (
          <AboutRow label="Original language">
            {language.trim().toUpperCase()}
          </AboutRow>
        )}
        {budget != null && budget > 0 && (
          <AboutRow label="Budget">{formatCurrency(budget)}</AboutRow>
        )}
        {revenue != null && revenue > 0 && (
          <AboutRow label="Revenue">{formatCurrency(revenue)}</AboutRow>
        )}
      </dl>
    </section>
  );
}
