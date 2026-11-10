import * as React from "react"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

// Variants are plain records and one function (docs/design-system.md §1.6).
// Keep the base one literal: the motion contract test reads a transition and
// its motion-reduce escape from the same string.
const BUTTON_BASE_CLASS =
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-sm font-medium whitespace-nowrap outline-hidden transition-[background-color,border-color,color,box-shadow,opacity] duration-micro focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 motion-reduce:transition-none dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"

const BUTTON_VARIANTS = {
  default:
    "bg-primary text-primary-foreground hover:bg-primary/90 disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100",
  destructive:
    "bg-destructive text-destructive-foreground hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40",
  outline:
    "border bg-background text-foreground shadow-xs hover:bg-accent hover:text-accent-foreground dark:border-border dark:bg-muted/90 dark:text-foreground dark:shadow-none dark:hover:bg-accent dark:hover:text-foreground [&_svg]:text-current",
  ghost: "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
  accent:
    "bg-primary text-primary-foreground shadow-md hover:bg-primary/90 disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none",
  "accent-pill":
    "rounded-full bg-primary text-primary-foreground shadow-md hover:bg-primary/90 disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none",
} as const

// Sizes set no radius: the base rounds every button and `accent-pill` alone
// overrides it, so the pill keeps its shape at every size.
const BUTTON_SIZES = {
  default: "h-9 px-4 py-2 has-[>svg]:px-3",
  sm: "h-8 gap-1.5 px-3 has-[>svg]:px-2.5",
  lg: "h-10 px-6 has-[>svg]:px-4",
  icon: "size-9",
  "icon-sm": "size-8",
} as const

type ButtonVariant = keyof typeof BUTTON_VARIANTS
type ButtonSize = keyof typeof BUTTON_SIZES

type ButtonVariantOptions = {
  variant?: ButtonVariant
  size?: ButtonSize
  className?: string
}

/**
 * The class string of a button, for an element that is not a `Button` (a
 * link, a Radix trigger). Later classes win: the size over the variant over
 * the base, and `className` over all three.
 */
function buttonVariants({
  variant = "default",
  size = "default",
  className,
}: ButtonVariantOptions = {}) {
  return cn(BUTTON_BASE_CLASS, BUTTON_VARIANTS[variant], BUTTON_SIZES[size], className)
}

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  Omit<ButtonVariantOptions, "className"> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={buttonVariants({ variant, size, className })}
      {...props}
    />
  )
}

export { Button, buttonVariants }
