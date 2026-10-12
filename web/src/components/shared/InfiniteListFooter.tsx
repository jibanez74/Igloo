import { Spinner } from "@/components/ui/spinner";
import LoadErrorAlert from "@/components/shared/LoadErrorAlert";

type InfiniteListFooterProps = {
  isFetchingNextPage: boolean;
  /** Set while the last next-page request failed; the list stays above it. */
  loadMoreError: string | null;
  onRetry: () => void;
};

/**
 * The tail of an infinite list: a spinner while the next page loads, and a
 * Try again alert when it failed, so a failed page never looks like the end
 * of the list. Scrolling back to the bottom retries too.
 */
export default function InfiniteListFooter({
  isFetchingNextPage,
  loadMoreError,
  onRetry,
}: InfiniteListFooterProps) {
  if (isFetchingNextPage) {
    return (
      <div className="flex justify-center py-4">
        <Spinner className="size-6 text-primary" />
      </div>
    );
  }

  if (loadMoreError) {
    return (
      <div className="p-4">
        <LoadErrorAlert message={loadMoreError} onRetry={onRetry} />
      </div>
    );
  }

  return null;
}
