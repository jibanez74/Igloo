import AboutSection, { AboutRow } from "@/components/shared/AboutSection";
import { formatCurrency, formatLanguageName } from "@/lib/format";
import { trimmedOrNull } from "@/lib/nullable";
import type { LibraryMovieProductionCompanyType } from "@/types/movies";

type MovieAboutSectionProps = {
  movieTitle: string;
  status?: string | null;
  language: string | null;
  budget: number | null;
  revenue: number | null;
  companies: LibraryMovieProductionCompanyType[];
};

export default function MovieAboutSection({
  movieTitle,
  status,
  language,
  budget,
  revenue,
  companies,
}: MovieAboutSectionProps) {
  const statusLabel = trimmedOrNull(status);
  const languageLabel = formatLanguageName(language ?? undefined);
  const companyNames = companies.map(pc => pc.name).join(", ");
  const hasBudget = budget != null && budget > 0;
  const hasRevenue = revenue != null && revenue > 0;

  // An unmatched movie has none of these; a heading over nothing reads as
  // missing content.
  if (!companyNames && !statusLabel && !languageLabel && !hasBudget && !hasRevenue) {
    return null;
  }

  return (
    <AboutSection title={movieTitle}>
      {companyNames !== "" && (
        <AboutRow label="Production">{companyNames}</AboutRow>
      )}
      {statusLabel && <AboutRow label="Status">{statusLabel}</AboutRow>}
      {languageLabel && (
        <AboutRow label="Original language">{languageLabel}</AboutRow>
      )}
      {hasBudget && <AboutRow label="Budget">{formatCurrency(budget)}</AboutRow>}
      {hasRevenue && (
        <AboutRow label="Revenue">{formatCurrency(revenue)}</AboutRow>
      )}
    </AboutSection>
  );
}
