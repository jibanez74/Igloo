import * as React from "react"

import {
  FIELD_VARIANTS,
  type FieldVariant,
} from "@/components/ui/field-variants"
import { cn } from "@/lib/utils"

type TextareaProps = React.ComponentProps<"textarea"> & {
  variant?: FieldVariant
}

// A multi-line text field in the looks Input takes. It resizes vertically as
// the browser allows; pass `resize-none` where the layout cannot give.
function Textarea({ className, variant = "filled", ...props }: TextareaProps) {
  return (
    <textarea
      data-slot="textarea"
      data-variant={variant}
      className={cn(
        "w-full rounded-md border border-input px-3 py-2 text-sm outline-hidden transition-[color,box-shadow] duration-micro placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none",
        "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
        "aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        FIELD_VARIANTS[variant],
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
