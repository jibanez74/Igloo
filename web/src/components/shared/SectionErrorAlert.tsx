import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

/**
 * Shared destructive alert for a section that failed to load as a whole: a
 * home section, the watch-room list, and the dead ends `MediaNotFound` renders
 * (design-system §3.4). `title={null}` drops the title when a page heading
 * already names the failure.
 */
export default function SectionErrorAlert({
  title = "Error",
  message,
}: {
  title?: string | null;
  message: string;
}) {
  return (
    <Alert className="border-destructive/25 bg-destructive/10 text-destructive">
      <AlertCircle className="size-4" aria-hidden="true" />
      {title != null && <AlertTitle>{title}</AlertTitle>}
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}
