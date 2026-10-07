import type { LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";

type LibraryEmptyStateProps = {
  icon: LucideIcon;
  message: string;
  /** Spacing overrides for tight hosts, such as a popover body. */
  className?: string;
};

// The minimal empty variant (design-system §3.4): a faded icon and one
// sentence, for a tab or filter that simply has nothing in it. Empty states
// with a call to action use EmptyState instead.
export default function LibraryEmptyState({
  icon: Icon,
  message,
  className,
}: LibraryEmptyStateProps) {
  return (
    <div className={cn("py-12 text-center text-muted-foreground", className)}>
      <Icon className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
      <p>{message}</p>
    </div>
  );
}
