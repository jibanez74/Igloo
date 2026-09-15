import type { LucideIcon } from "lucide-react";

type LibraryEmptyStateProps = {
  icon: LucideIcon;
  message: string;
};

// The minimal empty variant (design-system §3.4): a faded icon and one
// sentence, for a tab or filter that simply has nothing in it. Empty states
// with a call to action use EmptyState instead.
export default function LibraryEmptyState({
  icon: Icon,
  message,
}: LibraryEmptyStateProps) {
  return (
    <div className="py-12 text-center text-muted-foreground">
      <Icon className="mx-auto mb-4 size-10 opacity-50" aria-hidden="true" />
      <p>{message}</p>
    </div>
  );
}
