import AboutSection, { AboutRow } from "@/components/shared/AboutSection";
import { TMDB_LOGO_SIZE } from "@/lib/constants";
import { formatDate } from "@/lib/format";
import { trimmedOrNull, unwrapString } from "@/lib/nullable";
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
  const originalNameLabel = trimmedOrNull(originalName);
  const showOriginalName =
    originalNameLabel != null && originalNameLabel !== name;
  const statusLabel = trimmedOrNull(status);
  const typeLabel = trimmedOrNull(type);
  const languageLabel = trimmedOrNull(language);
  const firstAired = trimmedOrNull(firstAirDate);
  const lastAired = trimmedOrNull(lastAirDate);

  return (
    <AboutSection title={name}>
      {showOriginalName && (
        <AboutRow label="Original name">{originalNameLabel}</AboutRow>
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
      {statusLabel && <AboutRow label="Status">{statusLabel}</AboutRow>}
      {typeLabel && <AboutRow label="Type">{typeLabel}</AboutRow>}
      {languageLabel && (
        <AboutRow label="Original language">
          {languageLabel.toUpperCase()}
        </AboutRow>
      )}
      {firstAired && (
        <AboutRow label="First aired">
          <time dateTime={firstAired}>{formatDate(firstAired)}</time>
        </AboutRow>
      )}
      {lastAired && (
        <AboutRow label="Last aired">
          <time dateTime={lastAired}>{formatDate(lastAired)}</time>
        </AboutRow>
      )}
    </AboutSection>
  );
}
