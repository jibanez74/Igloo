import type { ComponentType, ReactNode } from "react";
import type { LucideProps } from "lucide-react";
import { cn } from "@/lib/utils";

type EmptyStateProps = {
  /** Lucide icon rendered inside the gradient bubble. */
  icon: ComponentType<LucideProps>;
  title: string;
  description?: ReactNode;
  /** Optional call-to-action (e.g. a Button) rendered below the description. */
  action?: ReactNode;
  /** Wrap the state in the muted, bordered card surface (used for in-list empties). */
  bordered?: boolean;
  className?: string;
};

// Shared empty-state presentation: centered gradient bubble + icon + copy + optional CTA.
// Keeps the "no content yet" treatment consistent across the app.
export default function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  bordered = false,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center py-12 text-center sm:py-16",
        bordered && "rounded-xl border border-primary/10 bg-muted/30",
        className,
      )}
    >
      <div className="mb-5 flex size-20 items-center justify-center rounded-full bg-linear-to-br from-muted via-muted to-primary/30 shadow-lg shadow-primary/5 sm:size-24">
        <Icon className="size-8 text-primary/40 sm:size-10" aria-hidden="true" />
      </div>
      <h3 className="mb-2 text-xl font-semibold text-foreground">{title}</h3>
      {description && (
        <p
          className={cn(
            "max-w-sm text-muted-foreground",
            action && "mb-5 sm:mb-6",
          )}
        >
          {description}
        </p>
      )}
      {action}
    </div>
  );
}
