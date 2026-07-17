import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

/** Shared destructive alert for home-section load failures (design-system §3.4). */
export default function SectionErrorAlert({ message }: { message: string }) {
  return (
    <Alert
      variant="destructive"
      className="border-destructive/25 bg-destructive/10 text-destructive"
    >
      <AlertCircle className="size-4" aria-hidden="true" />
      <AlertTitle>Error</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}
