import type { ReactNode, Ref } from "react";
import { AlertCircle, ArrowLeft, RotateCcw } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { MOTION_MICRO_COLORS_CLASS } from "@/lib/constants";

type StatusAction = {
  id: string;
  label: string;
  ariaLabel: string;
  onClick: () => void;
  icon: "back" | "retry";
  variant?: "primary" | "secondary";
  buttonRef?: Ref<HTMLButtonElement>;
};

type MoviePlaybackStatusScreenProps = {
  title?: string;
  message: ReactNode;
  variant?: "loading" | "error";
  actions?: StatusAction[];
  containerRef?: Ref<HTMLDivElement>;
};

const EMPTY_STATUS_ACTIONS: StatusAction[] = [];

const primaryActionClass = `inline-flex items-center gap-2 rounded-full bg-primary px-6 py-3 font-semibold text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden ${MOTION_MICRO_COLORS_CLASS}`;
const secondaryActionClass = `inline-flex items-center gap-2 rounded-full border border-border px-6 py-3 font-semibold text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden ${MOTION_MICRO_COLORS_CLASS}`;

function StatusActionIcon({ icon }: { icon: StatusAction["icon"] }) {
  if (icon === "retry") {
    return <RotateCcw className="size-5" aria-hidden="true" />;
  }

  return <ArrowLeft className="size-5" aria-hidden="true" />;
}

export default function MoviePlaybackStatusScreen({
  title,
  message,
  variant = "error",
  actions = EMPTY_STATUS_ACTIONS,
  containerRef,
}: MoviePlaybackStatusScreenProps) {
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
          <Spinner className="mx-auto mb-6 size-10 text-primary" />
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
              <button
                key={action.id}
                type="button"
                ref={action.buttonRef}
                onClick={action.onClick}
                className={
                  action.variant === "secondary"
                    ? secondaryActionClass
                    : primaryActionClass
                }
                aria-label={action.ariaLabel}
              >
                <StatusActionIcon icon={action.icon} />
                {action.label}
              </button>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}
