import type { ReactNode } from "react";
import { DETAIL_SECTION_HEADING_CLASS } from "@/lib/constants";

/**
 * Compact fine-print block at the foot of a detail page (Netflix-style
 * "About" footer): a definition list of catalog facts. Pages supply the rows.
 */
export default function AboutSection({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <section
      className="mt-10 border-t border-border pt-6 sm:mt-12"
      aria-labelledby="details-heading"
    >
      <h2
        id="details-heading"
        tabIndex={-1}
        className={DETAIL_SECTION_HEADING_CLASS}
      >
        About {title}
      </h2>
      <dl className="max-w-3xl space-y-1.5 text-sm">{children}</dl>
    </section>
  );
}

export function AboutRow({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-wrap gap-x-2">
      <dt className="shrink-0 text-muted-foreground">{label}:</dt>
      <dd className="min-w-0 text-foreground">{children}</dd>
    </div>
  );
}
