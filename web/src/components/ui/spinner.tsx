import { Loader2Icon } from "lucide-react"

import { MOTION_SPINNER_STATE_CLASS } from "@/lib/constants"
import { cn } from "@/lib/utils"

function Spinner({ className, ...props }: React.ComponentProps<"svg">) {
  return (
    <Loader2Icon
      role="status"
      aria-label="Loading"
      className={cn("size-4", MOTION_SPINNER_STATE_CLASS, className)}
      {...props}
    />
  )
}

export { Spinner }
