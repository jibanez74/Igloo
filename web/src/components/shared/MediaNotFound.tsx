import { Link } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

// Widen the union when a new call site needs another destination — keeping it
// a literal union preserves TanStack Router's typed `to`.
type BackDestination = "/" | "/movies" | "/music";

export default function MediaNotFound({
  message,
  backTo,
  backLabel,
}: {
  message: string;
  backTo: BackDestination;
  backLabel: string;
}) {
  return (
    <div>
      <Alert className="border-destructive/20 bg-destructive/10 text-destructive">
        <AlertCircle className="size-4" aria-hidden="true" />
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>{message}</AlertDescription>
      </Alert>
      <Link
        to={backTo}
        className={cn(buttonVariants({ variant: "outline" }), "mt-4")}
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {backLabel}
      </Link>
    </div>
  );
}
