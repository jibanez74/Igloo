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
        className="mt-2 rounded-sm text-sm font-medium text-primary underline hover:text-primary/80 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-hidden"
      >
        Try again
      </button>
    </div>
  );
}
