import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertCircle } from "lucide-react";

export default function MediaNotFound({ message }: { message: string }) {
  return (
    <Alert
      variant="destructive"
      className="border-destructive/20 bg-destructive/10 text-destructive"
    >
      <AlertCircle className="size-4" aria-hidden="true" />
      <AlertTitle>Error</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}
