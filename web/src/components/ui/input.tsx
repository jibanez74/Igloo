import * as React from "react"

import {
  FIELD_SIZES,
  FIELD_VARIANTS,
  type FieldSize,
  type FieldVariant,
} from "@/components/ui/field-variants"
import { cn } from "@/lib/utils"

type InputProps = Omit<React.ComponentProps<"input">, "size"> & {
  variant?: FieldVariant
  /** The field's height. The native `size` attribute is not exposed. */
  size?: FieldSize
}

function Input({
  className,
  type,
  variant = "filled",
  size = "default",
  ...props
}: InputProps) {
  return (
    <input
      type={type}
      data-slot="input"
      data-variant={variant}
      className={cn(
        "h-9 w-full min-w-0 rounded-md border border-input bg-transparent px-3 py-1 text-base shadow-xs outline-hidden transition-[color,box-shadow] duration-micro selection:bg-primary selection:text-primary-foreground file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none md:text-sm dark:bg-input/30",
        "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
        "aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40",
        FIELD_VARIANTS[variant],
        FIELD_SIZES[size],
        className
      )}
      {...props}
    />
  )
}

export { Input }
