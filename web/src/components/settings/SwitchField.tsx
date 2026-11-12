import type { ReactNode, Ref } from "react";

import { Switch } from "@/components/ui/switch";
import { MOTION_SETTINGS_SURFACE_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type SwitchFieldProps = {
  id: string;
  label: string;
  description: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  icon: ReactNode;
  disabled?: boolean;
  /**
   * A save of this switch is in flight. Unlike `disabled` it keeps the
   * switch focusable (a disabled control drops keyboard focus), says so with
   * `aria-disabled`, and ignores a press until the save is back.
   */
  busy?: boolean;
  ref?: Ref<HTMLButtonElement>;
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
  busy = false,
  ref,
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
        <Switch
          ref={ref}
          checked={checked}
          onCheckedChange={next => {
            if (!busy) onCheckedChange(next);
          }}
          aria-labelledby={labelId}
          aria-describedby={descriptionId}
          aria-disabled={busy || undefined}
          disabled={disabled}
          className="mt-1"
        />
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
