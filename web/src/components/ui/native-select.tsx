import * as React from "react"

import {
  FIELD_VARIANTS,
  type FieldVariant,
} from "@/components/ui/field-variants"
import { cn } from "@/lib/utils"

type NativeSelectProps = React.ComponentProps<"select"> & {
  variant?: FieldVariant
}

// The browser's own <select>, styled as a field. Use it where the platform
// picker is the better control: on touch devices, where it opens the system
// wheel or sheet, and for short filter lists. `Select` is the styled listbox.
function NativeSelect({
  className,
  variant = "filled",
  ...props
}: NativeSelectProps) {
  return (
    <select
      data-slot="native-select"
      data-variant={variant}
      className={cn(
        "w-full rounded-md border border-input px-3 py-2 text-sm outline-hidden disabled:opacity-50",
        "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
        FIELD_VARIANTS[variant],
        className
      )}
      {...props}
    />
  )
}

export { NativeSelect }
