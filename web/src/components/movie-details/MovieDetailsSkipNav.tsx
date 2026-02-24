type Section = { id: string; label: string };

type MovieDetailsSkipNavProps = {
  sections: Section[];
};

const skipLinkClass =
  "rounded-sm px-2 py-1 text-amber-400 underline focus:ring-2 focus:ring-amber-400 focus:outline-none";

export default function MovieDetailsSkipNav({
  sections,
}: MovieDetailsSkipNavProps) {
  if (sections.length === 0) return null;

  return (
    <nav
      aria-label="Skip to section"
      className="sr-only mb-4 focus-within:not-sr-only"
    >
      <ul className="flex flex-wrap gap-2">
        {sections.map(({ id, label }) => (
          <li key={id}>
            <a href={`#${id}`} className={skipLinkClass}>
              {label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
