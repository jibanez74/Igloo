import type { ReactNode } from "react";

import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type SwitchFieldProps = {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  icon: ReactNode;
  disabled?: boolean;
};

/**
 * A labelled, described on/off switch row for Settings cards: a real
 * `role="switch"` button so it reads as a toggle, with the description tied
 * in through aria-describedby.
 */
export default function SwitchField({
  id,
  label,
  description,
  checked,
  onCheckedChange,
  icon,
  disabled,
}: SwitchFieldProps) {
  const labelId = `${id}-label`;
  const descriptionId = `${id}-description`;

  return (
    <div
      className={cn(
        "rounded-lg border border-border/50 bg-card/50 p-4 hover:border-border/70",
        MOTION_SETTINGS_SURFACE_CLASS,
      )}
    >
      <div className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-start gap-x-3 gap-y-2">
        <div
          className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted"
          aria-hidden="true"
        >
          {icon}
        </div>
        <p
          id={labelId}
          className="min-w-0 pt-1 text-sm font-medium text-foreground"
        >
          {label}
        </p>
        <button
          type="button"
          role="switch"
          aria-checked={checked}
          aria-labelledby={labelId}
          aria-describedby={descriptionId}
          disabled={disabled}
          onClick={() => onCheckedChange(!checked)}
          className={cn(
            "relative mt-1 h-6 w-11 shrink-0 rounded-full border disabled:cursor-not-allowed disabled:opacity-60",
            FOCUS_VISIBLE_RING_CLASS,
            MOTION_SETTINGS_SURFACE_CLASS,
            checked ? "border-primary bg-primary" : "border-border bg-accent",
          )}
        >
          <span
            aria-hidden="true"
            className={cn(
              "absolute top-1/2 left-0 size-4 -translate-y-1/2 rounded-full bg-white shadow-sm",
              MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
              checked ? "translate-x-5" : "translate-x-1",
            )}
          />
        </button>
        <p
          id={descriptionId}
          className="col-span-3 text-sm text-muted-foreground min-[380px]:col-span-1 min-[380px]:col-start-2"
        >
          {description}
        </p>
      </div>
    </div>
  );
}
