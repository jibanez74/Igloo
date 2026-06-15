import type { ReactNode, Ref } from "react";
import { AlertCircle, ArrowLeft, RotateCcw } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";

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

const primaryActionClass =
  "inline-flex items-center gap-2 rounded-full bg-cyan-500 px-6 py-3 font-semibold text-slate-900 shadow-lg shadow-cyan-500/20 transition-colors hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none";
const secondaryActionClass =
  "inline-flex items-center gap-2 rounded-full border border-slate-600 px-6 py-3 font-semibold text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none";

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
      className="flex min-h-screen flex-col items-center justify-center bg-slate-900 px-4"
    >
      <div className={isLoading ? "text-center" : "max-w-md text-center"}>
        {isLoading ? (
          <Spinner className="mx-auto mb-6 size-10 text-cyan-400" />
        ) : (
          <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-red-500/10">
            <AlertCircle className="size-10 text-red-400" aria-hidden="true" />
          </div>
        )}

        {title ? (
          <h1 className="mb-2 text-xl font-semibold text-white">{title}</h1>
        ) : null}
        <p
          className={
            isLoading
              ? "text-lg font-medium text-white"
              : "mb-6 text-slate-400"
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
