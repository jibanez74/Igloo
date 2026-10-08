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
  /** Whether the form differs from what is saved; drives sticky and enabled. */
  isDirty: boolean;
  statusMessage: string;
  statusTone?: SaveBarTone;
  /** Set when a form field references the status text via aria-describedby. */
  statusId?: string;
  onReset: () => void;
  resetLabel?: string;
  /** Overrides the derived state (disabled while clean or pending). */
  resetDisabled?: boolean;
  saveLabel?: string;
  savingLabel?: string;
  /** Overrides the derived state (disabled while clean or pending). */
  saveDisabled?: boolean;
  isPending: boolean;
  /**
   * The bottom offset for the sticky bar (`bottom-4`, or the mini-player
   * clearance). Applied only while dirty; omit for a bar that never sticks.
   */
  stickyClassName?: string;
  /** Sit inside a Card as its footer: no own border, radius or shadow. */
  embedded?: boolean;
  /** Motion and other outer-wrapper classes. */
  className?: string;
};

const TONE_CLASS: Record<SaveBarTone, string> = {
  neutral: "text-muted-foreground",
  error: "text-destructive",
  success: "text-success",
};

const CLEAN_STATUS = "No unsaved changes";

/**
 * The shared Settings save bar: a status line (title + `aria-live` message)
 * and a Reset + Save action cluster, on an opaque card surface. It is always
 * rendered at the end of its form, so it can be found; it lifts into a sticky
 * bar only while there are unsaved edits, and its buttons are disabled when
 * there is nothing to save or reset (design-system §3.7). Below `sm` it is
 * one compact row: status only, an icon-only Reset, and Save.
 */
export default function SettingsSaveBar({
  title,
  isDirty,
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
  stickyClassName,
  embedded = false,
  className,
}: SettingsSaveBarProps) {
  // A success or error message outlives the edit that caused it ("Library
  // paths saved."), so only a neutral, clean bar reads as having nothing to do.
  const resolvedStatus =
    isDirty || statusTone !== "neutral" ? statusMessage : CLEAN_STATUS;
  const actionsDisabled = !isDirty || isPending;

  return (
    <div
      data-dirty={isDirty || undefined}
      className={cn(
        "flex items-center justify-between gap-3 bg-card p-3 sm:gap-4 sm:p-4",
        embedded
          ? "border-t border-border/50 bg-transparent px-6 pb-0 sm:px-6 sm:pb-0"
          : "rounded-lg border border-border/50 shadow-lg shadow-black/10",
        isDirty && stickyClassName && cn("sticky z-10", stickyClassName),
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <p className="hidden truncate text-sm font-medium text-foreground sm:block">
          {title}
        </p>
        <p
          id={statusId}
          className={cn(
            MOTION_MICRO_COLORS_CLASS,
            "truncate text-xs sm:mt-1 sm:text-sm sm:whitespace-normal",
            TONE_CLASS[statusTone],
          )}
          aria-live="polite"
        >
          {resolvedStatus}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={onReset}
          disabled={resetDisabled ?? actionsDisabled}
          className={cn(SETTINGS_RESET_BUTTON_CLASS, "px-3 sm:px-4")}
        >
          <RotateCcw className="size-4" aria-hidden="true" />
          <span className="sr-only sm:not-sr-only">{resetLabel}</span>
        </Button>
        <Button
          type="submit"
          variant="accent"
          disabled={saveDisabled ?? actionsDisabled}
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
