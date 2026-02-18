import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { AlertCircle } from "lucide-react";

export default function MediaNotFound({ message }: { message: string }) {
  return (
    <Alert
      variant="destructive"
      className="border-red-500/20 bg-red-500/10 text-red-400"
    >
      <AlertCircle className="size-4" aria-hidden="true" />
      <AlertTitle>Error</AlertTitle>
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  );
}
