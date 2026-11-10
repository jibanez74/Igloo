import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

type SettingsErrorCardProps = {
  title: string;
  message: string;
  /** Extra classes for the outer max-width wrapper. */
  className?: string;
};

/**
 * The shared "settings unavailable" state (design-system §3.4): a tinted
 * destructive Card with an `<h2>` title and the failure message. Used when a
 * settings query fails to load.
 */
export default function SettingsErrorCard({
  title,
  message,
  className,
}: SettingsErrorCardProps) {
  return (
    <div className={cn("max-w-3xl", className)} role="alert">
      <Card surface="error">
        <CardHeader>
          <CardTitle asChild tone="destructive">
            <h2>{title}</h2>
          </CardTitle>
          <CardDescription className="text-destructive">
            {message}
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  );
}
