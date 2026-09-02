import * as React from "react"
import { Separator as SeparatorPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Separator({
  className,
  ...props
}: Omit<
  React.ComponentProps<typeof SeparatorPrimitive.Root>,
  "decorative" | "orientation"
>) {
  return (
    <SeparatorPrimitive.Root
      data-slot="separator"
      decorative
      orientation="horizontal"
      className={cn("h-px w-full shrink-0 bg-border", className)}
      {...props}
    />
  )
}

export { Separator }
