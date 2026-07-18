type MusicianDetailsSkipLinksProps = {
  hasDiscography: boolean;
  hasTracks: boolean;
};

const linkClass =
  "rounded-sm px-2 py-1 text-primary underline focus:ring-2 focus:ring-ring focus:outline-hidden";

export default function MusicianDetailsSkipLinks({
  hasDiscography,
  hasTracks,
}: MusicianDetailsSkipLinksProps) {
  return (
    <nav
      aria-label="Skip to section"
      className="sr-only focus-within:not-sr-only"
    >
      <ul className="mb-4 flex flex-wrap gap-2">
        <li>
          <a href="#musician-name" className={linkClass}>
            Skip to musician info
          </a>
        </li>
        {hasDiscography && (
          <li>
            <a href="#discography-heading" className={linkClass}>
              Skip to discography
            </a>
          </li>
        )}
        {hasTracks && (
          <li>
            <a href="#tracks-heading" className={linkClass}>
              Skip to all tracks
            </a>
          </li>
        )}
      </ul>
    </nav>
  );
}
