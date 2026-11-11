import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { MOTION_SECTION_ENTER_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type PageHeaderProps = {
  /** Drawn in the primary color before the title; decorative. */
  icon: LucideIcon;
  /** The page's `h1`. */
  title: ReactNode;
  /** One line under the title saying what the page is for. */
  description?: ReactNode;
  /** Facts about the page beside its title: a library's counts. */
  meta?: ReactNode;
  /** The page's own controls, at the end of the title's row. */
  actions?: ReactNode;
  className?: string;
};

/**
 * The heading block that opens a top-level page (the libraries, search): an
 * icon and the `h1`, with a library's counts and its menu on the same row,
 * an optional line under them, and the section entrance. It holds the
 * page's one `h1`.
 */
export default function PageHeader({
  icon: Icon,
  title,
  description,
  meta,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS, className)}>
      {/* One row: the title, what there is of it, and what can be done with
          it. On a phone the counts do not fit beside the title, so they take
          a line of their own under it and the controls stay on the title's
          line. The order in the markup is the reading order throughout. */}
      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <h1 className="order-1 flex min-w-0 flex-1 items-center gap-3 text-3xl font-semibold tracking-tight text-foreground sm:flex-none md:text-4xl">
          <Icon className="size-6 shrink-0 text-primary" aria-hidden="true" />
          <span>{title}</span>
        </h1>
        {meta ? (
          <div className="order-3 min-w-0 basis-full sm:order-2 sm:basis-auto">
            {meta}
          </div>
        ) : null}
        {actions ? (
          <div className="order-2 ml-auto shrink-0 sm:order-3">{actions}</div>
        ) : null}
      </div>
      {description ? (
        <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
          {description}
        </p>
      ) : null}
    </header>
  );
}
