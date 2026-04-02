/** Shared load-error UI for movies routes (library, playlist detail, etc.). */

export function isApiFailure(data: unknown): data is { error: true; message: string } {
  return (
    typeof data === "object" &&
    data !== null &&
    "error" in data &&
    (data as { error: unknown }).error === true &&
    "message" in data &&
    typeof (data as { message: string }).message === "string"
  );
}

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
        onClick={() => {
          onRetry();
        }}
        className="mt-2 text-sm font-medium text-amber-400 underline hover:text-amber-300"
      >
        Try again
      </button>
    </div>
  );
}
