import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * The static help for a field. It sits between the label and the control, so
 * it is read before the control is used, and the control points at it with
 * `aria-describedby` (docs/design-system.md §3.7).
 */
function FieldHint({ className, ...props }: React.ComponentProps<"p">) {
  return (
    <p
      data-slot="field-hint"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

/**
 * A field's validation message. It sits below the control and is announced
 * when it appears; the control adds its id to `aria-describedby`.
 */
function FieldError({ className, ...props }: React.ComponentProps<"p">) {
  return (
    <p
      data-slot="field-error"
      role="alert"
      className={cn("text-sm text-destructive", className)}
      {...props}
    />
  );
}

export { FieldError, FieldHint };
