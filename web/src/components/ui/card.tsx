import * as React from "react"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

// `muted` is the panel the Settings pages are built from: a faint well on
// the canvas. `raised` is the opaque card that stands off its background.
// `error` holds a failure in place of a card's content; `danger` frames
// actions that remove data. They differ only in how strong the border is.
const CARD_SURFACES = {
  muted: "border-border/50 bg-muted/30",
  raised: "bg-card",
  error: "border-destructive/20 bg-destructive/10",
  danger: "border-destructive/50 bg-destructive/10",
} as const

function Card({
  className,
  surface = "muted",
  ...props
}: React.ComponentProps<"div"> & {
  surface?: keyof typeof CARD_SURFACES
}) {
  return (
    <div
      data-slot="card"
      data-surface={surface}
      className={cn(
        "flex flex-col gap-6 rounded-xl border py-6 text-card-foreground shadow-sm",
        CARD_SURFACES[surface],
        className
      )}
      {...props}
    />
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        "@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-2 px-6 [.border-b]:pb-6",
        className
      )}
      {...props}
    />
  )
}

function CardTitle({
  className,
  asChild = false,
  tone = "default",
  ...props
}: React.ComponentProps<"div"> & {
  asChild?: boolean
  /** `destructive` for the title of a card whose actions remove data. */
  tone?: "default" | "destructive"
}) {
  // Pass asChild with a heading child (e.g. <h2>) so card sections are
  // navigable by heading — see docs/design-system.md §3.7.
  const Comp = asChild ? Slot.Root : "div"
  return (
    <Comp
      data-slot="card-title"
      className={cn(
        "leading-none font-semibold",
        tone === "destructive" && "text-destructive",
        className
      )}
      {...props}
    />
  )
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-content"
      className={cn("px-6", className)}
      {...props}
    />
  )
}

function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-footer"
      className={cn("flex items-center px-6 [.border-t]:pt-6", className)}
      {...props}
    />
  )
}

export {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
}
