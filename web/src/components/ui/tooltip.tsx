import { createContext, use } from "react"
import type { ComponentProps, ReactElement, ReactNode } from "react"
import { Slot, Tooltip as TooltipPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

const InsideTooltipProvider = createContext(false)

/**
 * Shares the open delay between every tooltip under it: once one has shown,
 * moving to the next control shows its tooltip at once. The app mounts one
 * at its root; a `Tooltip` outside any provider brings its own.
 */
function TooltipProvider({
  delayDuration = 500,
  ...props
}: ComponentProps<typeof TooltipPrimitive.Provider>) {
  return (
    <InsideTooltipProvider value={true}>
      <TooltipPrimitive.Provider delayDuration={delayDuration} {...props} />
    </InsideTooltipProvider>
  )
}

// Radix describes the trigger with the tooltip while it is open. For a
// tooltip that only shows the control's own name, that reads the name twice,
// so this drops the attribute on its way to the control.
function Undescribed(props: ComponentProps<typeof Slot.Root>) {
  const rest = { ...props }
  delete rest["aria-describedby"]

  return <Slot.Root {...rest} />
}

type TooltipProps = {
  /** What the tooltip says. */
  label: ReactNode
  /** The control it belongs to: one element that can take focus. */
  children: ReactElement
  /**
   * Set when the tooltip adds to the control's name instead of repeating it
   * (the reason a button is disabled, a shortcut). It is then announced as
   * the control's description.
   */
  describes?: boolean
  side?: ComponentProps<typeof TooltipPrimitive.Content>["side"]
  align?: ComponentProps<typeof TooltipPrimitive.Content>["align"]
  /** Where to portal it: the fullscreen element, inside a fullscreen player. */
  container?: ComponentProps<typeof TooltipPrimitive.Portal>["container"]
  /** Forces it open or closed; for the gallery and tests. */
  open?: boolean
  /**
   * Shows nothing while set. The control keeps its place in the tree, so a
   * tooltip that stops applying (a reason that no longer holds) does not
   * remount the control and drop its focus.
   */
  disabled?: boolean
  className?: string
}

/**
 * A short label shown on hover and on keyboard focus, for a control whose
 * meaning is an icon. It never opens on touch, so it must not hold anything
 * a touch user needs: the control keeps its own `aria-label`.
 */
function Tooltip({
  label,
  children,
  describes = false,
  side = "top",
  align = "center",
  container,
  open,
  disabled = false,
  className,
}: TooltipProps) {
  const insideProvider = use(InsideTooltipProvider)

  const tooltip = (
    <TooltipPrimitive.Root open={open}>
      <TooltipPrimitive.Trigger asChild data-slot="tooltip-trigger">
        {describes ? children : <Undescribed>{children}</Undescribed>}
      </TooltipPrimitive.Trigger>
      {!disabled && (
        <TooltipPrimitive.Portal container={container}>
          <TooltipPrimitive.Content
            data-slot="tooltip-content"
            side={side}
            align={align}
            sideOffset={6}
            collisionPadding={8}
            className={cn(
              "z-popover w-fit max-w-64 origin-(--radix-tooltip-content-transform-origin) animate-in rounded-md bg-foreground px-2.5 py-1.5 text-xs font-medium text-balance text-background shadow-md duration-micro fade-in-0 zoom-in-95 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 motion-reduce:animate-none",
              className
            )}
          >
            {label}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      )}
    </TooltipPrimitive.Root>
  )

  return insideProvider ? tooltip : <TooltipProvider>{tooltip}</TooltipProvider>
}

export { Tooltip, TooltipProvider }
