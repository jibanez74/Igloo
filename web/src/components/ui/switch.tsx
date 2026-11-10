import type { ComponentProps } from "react"

import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
} from "@/lib/constants"
import { cn } from "@/lib/utils"

type SwitchProps = Omit<
  ComponentProps<"button">,
  "children" | "onChange" | "onClick" | "role" | "type"
> & {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
}

/**
 * An on/off switch: a real `role="switch"` button, so it reads as a toggle
 * and takes Space and Enter from the browser. It carries no label of its
 * own; name it with `aria-labelledby` or `aria-label`.
 */
function Switch({ checked, onCheckedChange, className, ...props }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      data-slot="switch"
      data-state={checked ? "checked" : "unchecked"}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative h-6 w-11 shrink-0 rounded-full border disabled:cursor-not-allowed disabled:opacity-60",
        FOCUS_VISIBLE_RING_CLASS,
        MOTION_SETTINGS_SURFACE_CLASS,
        // The off track is a mid grey in both themes so the white thumb
        // reads against it; bg-accent is near-white in light and hid it
        // (design-system §3.7).
        checked
          ? "border-primary bg-primary"
          : "border-border bg-muted-foreground/25",
        className
      )}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          "absolute top-1/2 left-0 size-4 -translate-y-1/2 rounded-full bg-white shadow-sm",
          MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
          checked ? "translate-x-5" : "translate-x-1"
        )}
      />
    </button>
  )
}

export { Switch }
