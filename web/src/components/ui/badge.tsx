import * as React from "react"

import { cn } from "@/lib/utils"

// Static pill/badge (count pills, rating chips, status tags). Non-interactive
// by design — never put onClick here; use Button for actionable chips. Plain
// variant record + cn() per the house cva policy (docs/design-system.md §1.6).
const badgeVariants = {
  default: "bg-primary text-primary-foreground",
  outline: "border border-border bg-background/60 text-muted-foreground",
}

type BadgeVariant = keyof typeof badgeVariants

function Badge({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"span"> & { variant?: BadgeVariant }) {
  return (
    <span
      data-slot="badge"
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
        badgeVariants[variant],
        className,
      )}
      {...props}
    />
  )
}

export { Badge }
