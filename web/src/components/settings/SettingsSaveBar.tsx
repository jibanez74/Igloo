import { RotateCcw, Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import {
  MOTION_MICRO_COLORS_CLASS,
  SETTINGS_RESET_BUTTON_CLASS,
} from "@/lib/constants";

type SaveBarTone = "neutral" | "error" | "success";

type SettingsSaveBarProps = {
  title: string;
  statusMessage: string;
  statusTone?: SaveBarTone;
  /** Set when a form field references the status text via aria-describedby. */
  statusId?: string;
  onReset: () => void;
  resetLabel?: string;
  resetDisabled?: boolean;
  saveLabel?: string;
  savingLabel?: string;
  /** Defaults to `isPending` when omitted. */
  saveDisabled?: boolean;
  isPending: boolean;
  /** Outer-wrapper background / position / motion classes. */
  className?: string;
};

const TONE_CLASS: Record<SaveBarTone, string> = {
  neutral: "text-muted-foreground",
  error: "text-destructive",
  success: "text-success",
};

/**
 * The shared Settings save bar: a status line (title + `aria-live` message) and
 * a Reset + Save action cluster. The outer wrapper's background, sticky
 * positioning, and motion vary per page and come in via `className`.
 */
export default function SettingsSaveBar({
  title,
  statusMessage,
  statusTone = "neutral",
  statusId,
  onReset,
  resetLabel = "Reset",
  resetDisabled,
  saveLabel = "Save Settings",
  savingLabel = "Saving...",
  saveDisabled,
  isPending,
  className,
}: SettingsSaveBarProps) {
  return (
    <div
      className={cn(
        "rounded-lg border border-border/50 p-4 shadow-lg shadow-black/10 sm:flex sm:items-center sm:justify-between sm:gap-4",
        className,
      )}
    >
      <div className="min-w-0">
        <p className="text-sm font-medium text-foreground">{title}</p>
        <p
          id={statusId}
          className={cn(
            MOTION_MICRO_COLORS_CLASS,
            "mt-1 text-sm",
            TONE_CLASS[statusTone],
          )}
          aria-live="polite"
        >
          {statusMessage}
        </p>
      </div>
      <div className="mt-4 flex flex-col gap-2 sm:mt-0 sm:flex-row">
        <Button
          type="button"
          variant="outline"
          onClick={onReset}
          disabled={resetDisabled}
          className={SETTINGS_RESET_BUTTON_CLASS}
        >
          <RotateCcw className="size-4" aria-hidden="true" />
          {resetLabel}
        </Button>
        <Button
          type="submit"
          variant="accent"
          disabled={saveDisabled ?? isPending}
        >
          {isPending ? (
            <Spinner className="size-4" aria-hidden="true" />
          ) : (
            <Save className="size-4" aria-hidden="true" />
          )}
          {isPending ? savingLabel : saveLabel}
        </Button>
      </div>
    </div>
  );
}
