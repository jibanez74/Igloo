export function MoviesLoadError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <div
      className="rounded-lg border border-red-500/25 bg-red-950/40 px-4 py-3 text-sm text-red-100"
      role="alert"
    >
      <p>{message}</p>
      <button
        type="button"
        onClick={onRetry}
        className="mt-2 text-sm font-medium text-primary underline hover:text-primary/80"
      >
        Try again
      </button>
    </div>
  );
}
