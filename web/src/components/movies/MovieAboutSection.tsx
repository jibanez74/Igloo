import AboutSection, { AboutRow } from "@/components/shared/AboutSection";
import { formatCurrency } from "@/lib/format";
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
  const languageLabel = trimmedOrNull(language);
  const companyNames = companies.map(pc => pc.name).join(", ");

  return (
    <AboutSection title={movieTitle}>
      {companyNames !== "" && (
        <AboutRow label="Production">{companyNames}</AboutRow>
      )}
      {statusLabel && <AboutRow label="Status">{statusLabel}</AboutRow>}
      {languageLabel && (
        <AboutRow label="Original language">
          {languageLabel.toUpperCase()}
        </AboutRow>
      )}
      {budget != null && budget > 0 && (
        <AboutRow label="Budget">{formatCurrency(budget)}</AboutRow>
      )}
      {revenue != null && revenue > 0 && (
        <AboutRow label="Revenue">{formatCurrency(revenue)}</AboutRow>
      )}
    </AboutSection>
  );
}
