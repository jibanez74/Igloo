import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * The one "Try again" control every load-failure surface renders (design-system
 * §3.4): a text link that hands the query's `refetch` back to the reader.
 * Underlined at rest: it sits in an alert's text, where colour alone would
 * not mark it as a control.
 */
export default function RetryButton({
  onRetry,
  className,
}: {
  onRetry: () => void;
  className?: string;
}) {
  return (
    <Button
      variant="link"
      size="inline"
      onClick={onRetry}
      className={cn("mt-2 underline", className)}
    >
      Try again
    </Button>
  );
}
