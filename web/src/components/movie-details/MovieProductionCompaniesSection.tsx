import type { MovieProductionCompanyView } from "@/lib/movie-details-view";

type MovieProductionCompaniesSectionProps = {
  companies: MovieProductionCompanyView[];
};

export default function MovieProductionCompaniesSection({
  companies,
}: MovieProductionCompaniesSectionProps) {
  if (!companies.length) return null;

  return (
    <section
      className="mt-10 animate-in fade-in"
      aria-labelledby="production-companies-heading"
    >
      <h2
        id="production-companies-heading"
        className="mb-4 text-xl font-semibold text-white"
        tabIndex={-1}
      >
        Production Companies
      </h2>
      <ul
        className="flex flex-wrap items-center gap-4"
        aria-label="Production companies"
      >
        {companies.map((company) => (
          <li
            key={company.id}
            className="flex items-center gap-2 rounded-lg border border-slate-700/50 bg-slate-800/50 px-3 py-2 transition-colors hover:border-amber-500/20"
          >
            {company.logoUrl ? (
              <img
                src={company.logoUrl}
                alt=""
                className="h-8 w-auto max-w-20 object-contain"
              />
            ) : null}
            <span className="text-sm font-medium text-white">
              {company.name}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
