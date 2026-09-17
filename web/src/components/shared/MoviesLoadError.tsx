import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

export function MoviesLoadError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div
      className="rounded-lg border border-destructive/25 bg-destructive/10 px-4 py-3 text-sm text-destructive"
      role="alert"
    >
      <p>{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className={cn(
          "mt-2 rounded-sm text-sm font-medium text-primary underline hover:text-primary/80",
          FOCUS_VISIBLE_RING_CLASS,
        )}
      >
        Try again
      </button>
    </div>
  );
}
