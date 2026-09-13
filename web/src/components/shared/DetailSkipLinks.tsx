import { SKIP_LINK_CLASS } from "@/lib/constants";

export type DetailSkipLink = {
  href: string;
  label: string;
};

type DetailSkipLinksProps = {
  /** The page title anchor, always first. */
  titleHref: string;
  titleLabel: string;
  /**
   * Section anchors in page order. A section that is not rendered passes
   * `false` so the list reads exactly what is on the page.
   */
  sections: (DetailSkipLink | false)[];
};

export default function DetailSkipLinks({
  titleHref,
  titleLabel,
  sections,
}: DetailSkipLinksProps) {
  return (
    <nav
      aria-label="Skip to section"
      className="sr-only focus-within:not-sr-only"
    >
      <ul className="mb-4 flex flex-wrap gap-2">
        <li>
          <a href={titleHref} className={SKIP_LINK_CLASS}>
            {titleLabel}
          </a>
        </li>
        {sections.map(
          section =>
            section && (
              <li key={section.href}>
                <a href={section.href} className={SKIP_LINK_CLASS}>
                  {section.label}
                </a>
              </li>
            ),
        )}
      </ul>
    </nav>
  );
}
