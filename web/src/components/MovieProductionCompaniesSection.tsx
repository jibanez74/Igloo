import { TMDB_LOGO_SIZE } from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { unwrapString } from "@/lib/nullable";
import type { MovieProductionCompaniesSectionProps } from "@/types";

export default function MovieProductionCompaniesSection({
  companies,
}: MovieProductionCompaniesSectionProps) {
  if (companies.length === 0) return null;

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="companies-heading">
      <h2
        id="companies-heading"
        tabIndex={-1}
        className="mb-4 text-xl font-semibold text-white outline-none sm:text-2xl"
      >
        Production Companies
      </h2>
      <ul className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:gap-4 md:gap-6">
        {companies.map(pc => {
          const logoPath = unwrapString(pc.logo);
          const logoUrl = buildTmdbImageUrl(logoPath, TMDB_LOGO_SIZE);
          return (
            <li
              key={pc.id}
              className="flex min-w-0 items-center gap-3 rounded-lg border border-amber-500/10 bg-slate-800/50 px-4 py-3 sm:max-w-md"
            >
              {logoUrl ? (
                <img
                  src={logoUrl}
                  alt=""
                  className="h-8 w-auto max-w-24 object-contain"
                />
              ) : (
                <span className="text-sm text-slate-500">No logo</span>
              )}
              <span className="text-sm font-medium text-white">{pc.name}</span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
