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
  // Outline in the destructive colour, for a removal that is not the main
  // action of its surface. Destructive in both themes: it does not take
  // outline's dark fill and text.
  "outline-destructive":
    "border border-destructive/50 bg-background text-destructive shadow-xs hover:bg-destructive/10 hover:text-destructive dark:bg-transparent dark:shadow-none dark:hover:bg-destructive/20 [&_svg]:text-current",
  ghost: "hover:bg-accent hover:text-accent-foreground dark:hover:bg-accent/50",
  // Ghost that rests in the muted text colour: toolbar and row icons.
  "ghost-muted":
    "text-muted-foreground hover:bg-accent hover:text-foreground dark:hover:bg-accent/50",
  // Ghost-muted whose hover warns: a row's remove or delete icon.
  "ghost-destructive":
    "text-muted-foreground hover:bg-accent hover:text-destructive dark:hover:bg-accent/50",
  accent:
    "bg-primary text-primary-foreground shadow-md hover:bg-primary/90 disabled:bg-muted disabled:text-muted-foreground disabled:opacity-100 disabled:shadow-none",
  // Text that acts, in the primary colour: a retry inside an alert, "Mark
  // all read". Pair with size="inline".
  link: "text-primary underline-offset-4 hover:underline",
  // Quiet text that lights up on hover: the way back in a toolbar.
  "link-muted": "font-normal text-muted-foreground hover:text-primary",
} as const

// Sizes set no radius: the base rounds every button and the pill shape alone
// overrides it, so a pill keeps its shape at every size.
const BUTTON_SIZES = {
  default: "h-9 px-4 py-2 has-[>svg]:px-3",
  sm: "h-8 gap-1.5 px-3 has-[>svg]:px-2.5",
  lg: "h-10 px-6 has-[>svg]:px-4",
  // 44 px, the touch-target minimum: the hero actions of a detail page and
  // the home billboard, and the calls to action of a full-screen status.
  xl: "h-11 px-6 has-[>svg]:px-4",
  icon: "size-9",
  "icon-sm": "size-8",
  // No box of its own: a text button that sits in a line of text or a
  // toolbar, as tall as its text. For the link variants.
  inline: "p-0",
} as const

// A pill is a shape any variant can take: the primary and the outline
// buttons that sit side by side in a toolbar are both pills.
const BUTTON_SHAPES = {
  default: "",
  pill: "rounded-full",
} as const

type ButtonVariant = keyof typeof BUTTON_VARIANTS
type ButtonSize = keyof typeof BUTTON_SIZES
type ButtonShape = keyof typeof BUTTON_SHAPES

type ButtonVariantOptions = {
  variant?: ButtonVariant
  size?: ButtonSize
  shape?: ButtonShape
  className?: string
}

/**
 * The class string of a button, for an element that is not a `Button` (a
 * link, a Radix trigger). Later classes win: the shape over the size over
 * the variant over the base, and `className` over all four.
 */
function buttonVariants({
  variant = "default",
  size = "default",
  shape = "default",
  className,
}: ButtonVariantOptions = {}) {
  return cn(
    BUTTON_BASE_CLASS,
    BUTTON_VARIANTS[variant],
    BUTTON_SIZES[size],
    BUTTON_SHAPES[shape],
    className
  )
}

function Button({
  className,
  variant = "default",
  size = "default",
  shape = "default",
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
      data-shape={shape}
      className={buttonVariants({ variant, size, shape, className })}
      // A native button submits the form around it unless told otherwise,
      // so a Button that only acts (an invitee chip's remove, rendered
      // inside the create-room form) submitted it. A submit button says
      // type="submit"; asChild renders the child's own element.
      {...(asChild ? {} : { type: "button" as const })}
      {...props}
    />
  )
}

export { Button, buttonVariants }
