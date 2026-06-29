import type { MovieDetailsSkipLinksProps } from "@/types";

const linkClass =
  "rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-ring focus:outline-none";

export default function MovieDetailsSkipLinks({
  showCrewSection,
  castNonEmpty,
  chaptersNonEmpty,
  extrasNonEmpty,
  companiesNonEmpty,
}: MovieDetailsSkipLinksProps) {
  return (
    <nav
      aria-label="Skip to section"
      className="sr-only focus-within:not-sr-only"
    >
      <ul className="mb-4 flex flex-wrap gap-2">
        <li>
          <a href="#movie-title" className={linkClass}>
            Skip to movie info
          </a>
        </li>
        <li>
          <a href="#overview-heading" className={linkClass}>
            Skip to overview
          </a>
        </li>
        {showCrewSection && (
          <li>
            <a href="#crew-heading" className={linkClass}>
              Skip to key crew
            </a>
          </li>
        )}
        {castNonEmpty && (
          <li>
            <a href="#cast-heading" className={linkClass}>
              Skip to cast
            </a>
          </li>
        )}
        {chaptersNonEmpty && (
          <li>
            <a href="#chapters-heading" className={linkClass}>
              Skip to chapters
            </a>
          </li>
        )}
        <li>
          <a href="#details-heading" className={linkClass}>
            Skip to details
          </a>
        </li>
        {extrasNonEmpty && (
          <li>
            <a href="#extra-videos-heading" className={linkClass}>
              Skip to extra videos
            </a>
          </li>
        )}
        {companiesNonEmpty && (
          <li>
            <a href="#companies-heading" className={linkClass}>
              Skip to production companies
            </a>
          </li>
        )}
      </ul>
    </nav>
  );
}
