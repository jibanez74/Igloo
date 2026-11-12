import type { ReactNode, Ref } from "react";
import { AlertCircle, ArrowLeft, RotateCcw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

type StatusAction = {
  id: string;
  label: string;
  ariaLabel: string;
  onClick: () => void;
  icon: "back" | "retry";
  variant?: "primary" | "secondary";
  buttonRef?: Ref<HTMLButtonElement>;
};

type PlaybackStatusScreenProps = {
  title?: string;
  message: ReactNode;
  variant?: "loading" | "error";
  actions?: StatusAction[];
  containerRef?: Ref<HTMLDivElement>;
};

const EMPTY_STATUS_ACTIONS: StatusAction[] = [];

function StatusActionIcon({ icon }: { icon: StatusAction["icon"] }) {
  if (icon === "retry") {
    return <RotateCcw className="size-5" aria-hidden="true" />;
  }

  return <ArrowLeft className="size-5" aria-hidden="true" />;
}

export default function PlaybackStatusScreen({
  title,
  message,
  variant = "error",
  actions = EMPTY_STATUS_ACTIONS,
  containerRef,
}: PlaybackStatusScreenProps) {
  const isLoading = variant === "loading";

  return (
    <div
      ref={containerRef}
      className="flex min-h-screen flex-col items-center justify-center bg-background px-4"
    >
      {/* The player subtree (and its live regions) unmounts before this
          screen mounts, so error variants must self-announce; loading keeps
          the role="status" spinner convention. The key remounts this element
          across the loading -> error transition: every variant renders the same
          component at the same position, so without it React would reuse the
          node and add role="alert" to text that is already on screen, which
          screen readers do not announce. */}
      <div
        key={isLoading ? "loading" : "alert"}
        role={isLoading ? undefined : "alert"}
        className={isLoading ? "text-center" : "max-w-md text-center"}
      >
        {isLoading ? (
          <Spinner size="2xl" tone="primary" className="mx-auto mb-6" />
        ) : (
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-destructive/10">
            <AlertCircle className="size-10 text-destructive" aria-hidden="true" />
          </div>
        )}

        {title ? (
          <h1 className="mb-2 text-xl font-semibold text-foreground">{title}</h1>
        ) : null}
        <p
          className={
            isLoading
              ? "text-lg font-medium text-foreground"
              : "mb-6 text-muted-foreground"
          }
        >
          {message}
        </p>

        {actions.length > 0 ? (
          <div className="flex items-center justify-center gap-3">
            {actions.map((action) => (
              // The status screen's pair: a primary call to action with the
              // glow, and the quiet second action in outline.
              <Button
                key={action.id}
                ref={action.buttonRef}
                variant={action.variant === "secondary" ? "outline" : "default"}
                size="xl"
                shape="pill"
                onClick={action.onClick}
                className={cn(
                  "font-semibold",
                  action.variant !== "secondary" && "shadow-glow",
                )}
                aria-label={action.ariaLabel}
              >
                <StatusActionIcon icon={action.icon} />
                {action.label}
              </Button>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}
