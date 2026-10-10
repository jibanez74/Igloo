import RetryButton from "@/components/shared/RetryButton";

export default function LoadErrorAlert({
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
      <RetryButton onRetry={onRetry} />
    </div>
  );
}
