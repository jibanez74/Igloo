import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants"
import { cn } from "@/lib/utils"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(MOTION_LOADING_STATE_CLASS, "rounded-md bg-accent", className)}
      {...props}
    />
  )
}

export { Skeleton }
