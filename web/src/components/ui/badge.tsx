import * as React from "react"

import { cn } from "@/lib/utils"

// Static pill/badge (count pills, rating chips, status tags). Non-interactive
// by design — never put onClick here; use Button for actionable chips. Plain
// variant record + cn() per the house cva policy (docs/design-system.md §1.6).
const badgeVariants = {
  default: "bg-primary text-primary-foreground",
  outline: "border border-border bg-background/60 text-muted-foreground",
  // A quiet chip over a backdrop or poster: literal white on black, which
  // must not follow the theme (docs/design-system.md §1.2).
  "over-media": "border border-white/25 bg-black/30 text-white/90",
  // The two chips of a music detail hero: a fact about the album or artist,
  // and a genre.
  muted: "border border-border/40 bg-muted/90 text-foreground",
  tinted: "border border-primary/30 bg-muted/80 text-primary backdrop-blur-sm",
}

const badgeSizes = {
  sm: "px-2",
  default: "",
  lg: "px-3 py-1 text-sm",
}

type BadgeVariant = keyof typeof badgeVariants
type BadgeSize = keyof typeof badgeSizes

function Badge({
  className,
  variant = "default",
  size = "default",
  ...props
}: React.ComponentProps<"span"> & {
  variant?: BadgeVariant
  size?: BadgeSize
}) {
  return (
    <span
      data-slot="badge"
      data-variant={variant}
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-medium whitespace-nowrap",
        badgeVariants[variant],
        badgeSizes[size],
        className,
      )}
      {...props}
    />
  )
}

export { Badge }
