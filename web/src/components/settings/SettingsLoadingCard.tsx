import { useId } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import { SETTINGS_CARD_SURFACE_CLASS } from "@/lib/constants";

type SettingsLoadingCardProps = {
  label: string;
  /** Extra classes for the outer max-width wrapper. */
  className?: string;
};

/**
 * The shared settings loading state (design-system §3.4): a spinner centered in
 * a settings-surface Card under a single `role="status"` region, so the async
 * load is announced rather than conveyed by animation alone.
 */
export default function SettingsLoadingCard({
  label,
  className,
}: SettingsLoadingCardProps) {
  const loadingId = useId();

  return (
    <div
      className={cn("max-w-5xl", className)}
      role="status"
      aria-labelledby={loadingId}
    >
      <Card className={SETTINGS_CARD_SURFACE_CLASS}>
        <CardContent className="flex min-h-40 items-center justify-center">
          <div className="flex items-center gap-3 text-muted-foreground">
            <Spinner className="size-5 text-primary" aria-hidden="true" />
            <span id={loadingId}>{label}</span>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
