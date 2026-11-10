import type { ComponentProps } from "react"

import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants"
import { cn } from "@/lib/utils"

/**
 * One muted box of a loading layout. Its size, and a radius other than the
 * small default, come from `className`.
 *
 * It has no role and says nothing: the layout it belongs to is announced
 * once, by the `SkeletonStatus` around it (design-system §3.4). It does not
 * pulse by default either, since that wrapper usually pulses the whole
 * layout; `pulse` is for a box that loads on its own.
 */
function Skeleton({
  pulse = false,
  className,
  ...props
}: ComponentProps<"div"> & { pulse?: boolean }) {
  return (
    <div
      data-slot="skeleton"
      className={cn(
        "rounded-sm bg-muted",
        pulse && MOTION_LOADING_STATE_CLASS,
        className
      )}
      {...props}
    />
  )
}

export { Skeleton }
