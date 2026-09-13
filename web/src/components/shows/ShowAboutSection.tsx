import { TMDB_LOGO_SIZE } from "@/lib/constants";
import { formatDate } from "@/lib/format";
import { unwrapString } from "@/lib/nullable";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { ShowNetworkType, ShowProductionCompanyType } from "@/types";

type ShowAboutSectionProps = {
  name: string;
  originalName: string | null;
  status: string | null;
  type: string | null;
  language: string | null;
  firstAirDate: string | null;
  lastAirDate: string | null;
  networks: ShowNetworkType[];
  companies: ShowProductionCompanyType[];
};

function AboutRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap gap-x-2">
      <dt className="shrink-0 text-muted-foreground">{label}:</dt>
      <dd className="min-w-0 text-foreground">{children}</dd>
    </div>
  );
}

function hasText(value: string | null): value is string {
  return value != null && value.trim() !== "";
}

export default function ShowAboutSection({
  name,
  originalName,
  status,
  type,
  language,
  firstAirDate,
  lastAirDate,
  networks,
  companies,
}: ShowAboutSectionProps) {
  const companyNames = companies.map(pc => pc.name).join(", ");
  // The original name is only worth a row when it differs from the title shown.
  const showOriginalName = hasText(originalName) && originalName.trim() !== name;

  return (
    <section
      className="mt-10 border-t border-border pt-6 sm:mt-12"
      aria-labelledby="details-heading"
    >
      <h2
        id="details-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-foreground outline-hidden sm:text-xl"
      >
        About {name}
      </h2>
      <dl className="max-w-3xl space-y-1.5 text-sm">
        {showOriginalName && (
          <AboutRow label="Original name">{originalName.trim()}</AboutRow>
        )}
        {networks.length > 0 && (
          <AboutRow label="Network">
            <ul className="flex list-none flex-wrap items-center gap-x-3 gap-y-1">
              {networks.map(network => {
                const logoUrl = buildTmdbImageUrl(
                  unwrapString(network.logo),
                  TMDB_LOGO_SIZE,
                );

                return (
                  <li key={network.id} className="flex items-center gap-1.5">
                    {logoUrl !== "" && (
                      <img
                        src={logoUrl}
                        alt=""
                        loading="lazy"
                        decoding="async"
                        className="h-4 w-auto max-w-16 object-contain"
                      />
                    )}
                    <span>{network.name}</span>
                  </li>
                );
              })}
            </ul>
          </AboutRow>
        )}
        {companyNames !== "" && (
          <AboutRow label="Production">{companyNames}</AboutRow>
        )}
        {hasText(status) && <AboutRow label="Status">{status.trim()}</AboutRow>}
        {hasText(type) && <AboutRow label="Type">{type.trim()}</AboutRow>}
        {hasText(language) && (
          <AboutRow label="Original language">
            {language.trim().toUpperCase()}
          </AboutRow>
        )}
        {hasText(firstAirDate) && (
          <AboutRow label="First aired">
            <time dateTime={firstAirDate}>{formatDate(firstAirDate)}</time>
          </AboutRow>
        )}
        {hasText(lastAirDate) && (
          <AboutRow label="Last aired">
            <time dateTime={lastAirDate}>{formatDate(lastAirDate)}</time>
          </AboutRow>
        )}
      </dl>
    </section>
  );
}
