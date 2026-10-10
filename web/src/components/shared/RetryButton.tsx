import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

/**
 * The one "Try again" control every load-failure surface renders (design-system
 * §3.4): a text link that hands the query's `refetch` back to the reader.
 */
export default function RetryButton({
  onRetry,
  className,
}: {
  onRetry: () => void;
  className?: string;
}) {
  return (
    <button
      type="button"
      onClick={onRetry}
      className={cn(
        "mt-2 rounded-sm text-sm font-medium text-primary underline hover:text-primary/80",
        FOCUS_VISIBLE_RING_CLASS,
        className,
      )}
    >
      Try again
    </button>
  );
}
