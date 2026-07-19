const linkClass =
  "rounded-sm px-2 py-1 text-primary underline focus:ring-2 focus:ring-ring focus:outline-hidden";

export default function AlbumDetailsSkipLinks() {
  return (
    <nav
      aria-label="Skip to section"
      className="sr-only focus-within:not-sr-only"
    >
      <ul className="mb-4 flex flex-wrap gap-2">
        <li>
          <a href="#album-title" className={linkClass}>
            Skip to album info
          </a>
        </li>
        <li>
          <a href="#tracklist-heading" className={linkClass}>
            Skip to track list
          </a>
        </li>
        <li>
          <a href="#details-heading" className={linkClass}>
            Skip to album details
          </a>
        </li>
      </ul>
    </nav>
  );
}
