import { Loader2Icon } from "lucide-react"

import { MOTION_SPINNER_STATE_CLASS } from "@/lib/constants"
import { cn } from "@/lib/utils"

const SPINNER_SIZES = {
  sm: "size-4",
  md: "size-5",
  lg: "size-6",
  xl: "size-8",
  "2xl": "size-10",
} as const

// A spinner takes the text colour around it unless given a tone. A tone is
// important on purpose: inside an outline button, whose icons are forced to
// the button's colour, it is what keeps the spinner in its own.
const SPINNER_TONES = {
  current: "",
  primary: "text-primary!",
  success: "text-success!",
} as const

type SpinnerProps = React.ComponentProps<"svg"> & {
  size?: keyof typeof SPINNER_SIZES
  tone?: keyof typeof SPINNER_TONES
}

function Spinner({
  className,
  size = "sm",
  tone = "current",
  ...props
}: SpinnerProps) {
  return (
    <Loader2Icon
      role="status"
      aria-label="Loading"
      className={cn(
        SPINNER_SIZES[size],
        SPINNER_TONES[tone],
        MOTION_SPINNER_STATE_CLASS,
        className
      )}
      {...props}
    />
  )
}

export { Spinner }
