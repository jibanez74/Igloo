import type { ComponentProps, ReactNode } from "react"

import {
  MOTION_MICRO_COLORS_CLASS,
  PEER_FOCUS_VISIBLE_RING_CLASS,
} from "@/lib/constants"
import { cn } from "@/lib/utils"

import { Label } from "./label"

type RadioCardProps = Omit<
  ComponentProps<"input">,
  "aria-labelledby" | "checked" | "children" | "className" | "id" | "type"
> & {
  /** The radio's id; its label is `${id}-label`. */
  id: string
  checked: boolean
  /** The card's wrapper: `li` when the group is a list. */
  as?: "div" | "li"
  /** What the card says, inside the radio's label. */
  children: ReactNode
  /** Content under the label that is not part of the radio's name. */
  footer?: ReactNode
  className?: string
}

/**
 * A radio drawn as a whole card: the native input is kept, out of sight, so
 * the group has the browser's arrow-key behaviour, and the card is its label,
 * so a click anywhere on it picks it. The picked card is tinted and the
 * focused one ringed.
 */
function RadioCard({
  id,
  checked,
  as: Wrapper = "div",
  children,
  footer,
  className,
  ...props
}: RadioCardProps) {
  const labelId = `${id}-label`

  return (
    <Wrapper
      data-slot="radio-card"
      data-state={checked ? "checked" : "unchecked"}
      className={cn(
        MOTION_MICRO_COLORS_CLASS,
        "overflow-hidden rounded-lg border",
        checked
          ? "border-primary bg-primary/10"
          : "border-border bg-muted/60 hover:border-border",
        className
      )}
    >
      <input
        id={id}
        type="radio"
        checked={checked}
        aria-labelledby={labelId}
        className="peer sr-only"
        {...props}
      />
      <Label
        id={labelId}
        htmlFor={id}
        className={cn(
          "mb-0 flex cursor-pointer gap-3 rounded-lg p-2",
          PEER_FOCUS_VISIBLE_RING_CLASS
        )}
      >
        {children}
      </Label>
      {footer}
    </Wrapper>
  )
}

export { RadioCard }
