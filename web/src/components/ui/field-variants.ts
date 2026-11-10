import { lightInputClassName } from "@/lib/input-styles"

// The looks a text field can take, shared by Input, Textarea, NativeSelect
// and SelectTrigger so that a form mixing them reads as one set
// (docs/design-system.md §1.6, §3.7).
//
//   filled   the default: a muted well, for dialogs and pickers.
//   inset    a translucent well on a card, for the Settings forms.
//   glass    the light glass field of the header search, the sign-in page
//            and the account forms (src/lib/input-styles.ts).
//   outline  no fill of its own: the bare bordered field.
export const FIELD_VARIANTS = {
  filled: "border-border bg-muted text-foreground",
  inset: "border-border bg-background/60 text-foreground",
  glass: lightInputClassName,
  outline: "",
} as const

export type FieldVariant = keyof typeof FIELD_VARIANTS

// Heights of a single-line field. `default` adds nothing: each primitive's
// base already sets it.
export const FIELD_SIZES = {
  sm: "h-8",
  default: "",
  lg: "h-10",
} as const

export type FieldSize = keyof typeof FIELD_SIZES
