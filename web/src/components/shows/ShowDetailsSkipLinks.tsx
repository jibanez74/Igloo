type ShowDetailsSkipLinksProps = {
  seasonsNonEmpty: boolean;
  crewNonEmpty: boolean;
  castNonEmpty: boolean;
  extrasNonEmpty: boolean;
};

const linkClass =
  "rounded-sm px-2 py-1 text-primary underline focus:ring-2 focus:ring-ring focus:outline-hidden";

export default function ShowDetailsSkipLinks({
  seasonsNonEmpty,
  crewNonEmpty,
  castNonEmpty,
  extrasNonEmpty,
}: ShowDetailsSkipLinksProps) {
  return (
    <nav
      aria-label="Skip to section"
      className="sr-only focus-within:not-sr-only"
    >
      <ul className="mb-4 flex flex-wrap gap-2">
        <li>
          <a href="#show-title" className={linkClass}>
            Skip to show info
          </a>
        </li>
        <li>
          <a href="#overview-heading" className={linkClass}>
            Skip to overview
          </a>
        </li>
        {seasonsNonEmpty && (
          <li>
            <a href="#seasons-heading" className={linkClass}>
              Skip to seasons
            </a>
          </li>
        )}
        {crewNonEmpty && (
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
        {extrasNonEmpty && (
          <li>
            <a href="#extra-videos-heading" className={linkClass}>
              Skip to extra videos
            </a>
          </li>
        )}
        <li>
          <a href="#details-heading" className={linkClass}>
            Skip to about
          </a>
        </li>
      </ul>
    </nav>
  );
}
