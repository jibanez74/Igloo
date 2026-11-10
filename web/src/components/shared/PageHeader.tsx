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
  className?: string;
};

/**
 * The heading block that opens a top-level page (the libraries, search): an
 * icon and the `h1`, an optional line under them, and the section entrance.
 * It holds the page's one `h1`.
 */
export default function PageHeader({
  icon: Icon,
  title,
  description,
  className,
}: PageHeaderProps) {
  return (
    <header className={cn("mb-6 sm:mb-7", MOTION_SECTION_ENTER_CLASS, className)}>
      <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
        <Icon className="size-6 shrink-0 text-primary" aria-hidden="true" />
        <span>{title}</span>
      </h1>
      {description ? (
        <p className="mt-1.5 max-w-2xl text-sm text-muted-foreground md:text-base">
          {description}
        </p>
      ) : null}
    </header>
  );
}
