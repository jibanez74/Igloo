import * as React from "react"
import { MenuIcon, PanelLeftIcon } from "lucide-react"
import { Slot } from "radix-ui"

import { useIsMobile } from "@/hooks/use-mobile"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Tooltip } from "@/components/ui/tooltip"
import {
  SidebarContext,
  useSidebar,
} from "@/components/ui/sidebar-context"
import type { SidebarContextProps } from "./sidebar-context"

const SIDEBAR_WIDTH = "16rem"
const SIDEBAR_WIDTH_MOBILE = "18rem"
const SIDEBAR_WIDTH_ICON = "3rem"
const SIDEBAR_KEYBOARD_SHORTCUT = "b"
const SIDEBAR_STORAGE_KEY = "igloo-sidebar"

// The desktop column's state outlives the page, so a reader who collapsed it
// finds it collapsed after a reload. Storage can be unavailable (private
// mode, a blocked site): the column is then simply expanded each time.
function readStoredOpen() {
  try {
    return window.localStorage.getItem(SIDEBAR_STORAGE_KEY) !== "collapsed"
  } catch {
    return true
  }
}

function storeOpen(open: boolean) {
  try {
    window.localStorage.setItem(
      SIDEBAR_STORAGE_KEY,
      open ? "expanded" : "collapsed"
    )
  } catch {
    // Nothing to do: the state still holds for this page.
  }
}

function SidebarProvider({
  className,
  style,
  children,
  ...props
}: React.ComponentProps<"div">) {
  const isMobile = useIsMobile()
  const [open, setOpen] = React.useState(readStoredOpen)
  const [openMobile, setOpenMobile] = React.useState(false)

  React.useEffect(() => {
    storeOpen(open)
  }, [open])

  const toggleSidebar = () => {
    if (isMobile) {
      setOpenMobile((currentOpen) => !currentOpen)
    } else {
      setOpen((currentOpen) => !currentOpen)
    }
  }

  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (
        event.key === SIDEBAR_KEYBOARD_SHORTCUT &&
        (event.metaKey || event.ctrlKey)
      ) {
        event.preventDefault()
        if (isMobile) {
          setOpenMobile((currentOpen) => !currentOpen)
        } else {
          setOpen((currentOpen) => !currentOpen)
        }
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  })

  const contextValue: SidebarContextProps = {
    state: open ? "expanded" : "collapsed",
    isMobile,
    openMobile,
    setOpenMobile,
    toggleSidebar,
  }

  return (
    // The React Compiler memoizes contextValue; a manual useMemo here would go
    // against web/AGENTS.md.
    // react-doctor-disable-next-line react-doctor/context-provider-value-from-unmemoized-local-literal
    <SidebarContext.Provider value={contextValue}>
      <div
        data-slot="sidebar-wrapper"
        style={
          {
            "--sidebar-width": SIDEBAR_WIDTH,
            "--sidebar-width-icon": SIDEBAR_WIDTH_ICON,
            ...style,
          } as React.CSSProperties
        }
        className={cn("flex min-h-svh w-full", className)}
        {...props}
      >
        {children}
      </div>
    </SidebarContext.Provider>
  )
}

function Sidebar({
  className,
  children,
  ...props
}: React.ComponentProps<"div">) {
  const { isMobile, state, openMobile, setOpenMobile } = useSidebar()

  if (isMobile) {
    return (
      <Sheet open={openMobile} onOpenChange={setOpenMobile} {...props}>
        <SheetContent
          aria-describedby={undefined}
          data-sidebar="sidebar"
          data-slot="sidebar"
          data-mobile="true"
          className="w-(--sidebar-width) border-sidebar-border bg-sidebar p-0 text-sidebar-foreground"
          style={
            {
              "--sidebar-width": SIDEBAR_WIDTH_MOBILE,
            } as React.CSSProperties
          }
        >
          <SheetHeader className="sr-only">
            <SheetTitle>Navigation</SheetTitle>
          </SheetHeader>
          <div className="flex size-full flex-col">{children}</div>
        </SheetContent>
      </Sheet>
    )
  }

  return (
    <div
      className="group hidden text-sidebar-foreground md:block"
      data-state={state}
      data-collapsible={state === "collapsed" ? "icon" : ""}
      data-slot="sidebar"
    >
      <div
        data-slot="sidebar-gap"
        className="relative w-(--sidebar-width) bg-transparent transition-[width] duration-standard ease-linear group-data-[collapsible=icon]:w-(--sidebar-width-icon) motion-reduce:transition-none"
      />
      <div
        data-slot="sidebar-container"
        className={cn(
          "fixed inset-y-0 left-0 z-10 hidden h-svh w-(--sidebar-width) border-r transition-[width] duration-standard ease-linear group-data-[collapsible=icon]:w-(--sidebar-width-icon) motion-reduce:transition-none md:flex",
          className
        )}
        {...props}
      >
        <div
          data-sidebar="sidebar"
          data-slot="sidebar-inner"
          className="flex size-full flex-col bg-sidebar"
        >
          {children}
        </div>
      </div>
    </div>
  )
}

// One control for both shapes of the navigation: on a phone it opens the
// sheet (a menu icon, a 40px target); on a desktop it collapses the column to
// its icons and back, and says which in its name and tooltip.
function SidebarTrigger({
  className,
  onClick,
  ...props
}: React.ComponentProps<typeof Button>) {
  const { isMobile, openMobile, state, toggleSidebar } = useSidebar()
  const expanded = isMobile ? openMobile : state === "expanded"
  const label = isMobile
    ? "Open navigation"
    : expanded
      ? "Collapse sidebar"
      : "Expand sidebar"

  const button = (
    <Button
      data-sidebar="trigger"
      data-slot="sidebar-trigger"
      variant="ghost-muted"
      size="icon"
      aria-label={label}
      aria-expanded={expanded}
      className={cn(isMobile ? "size-10" : "size-8", className)}
      onClick={(event) => {
        onClick?.(event)
        toggleSidebar()
      }}
      {...props}
    >
      {isMobile ? (
        <MenuIcon aria-hidden="true" />
      ) : (
        <PanelLeftIcon aria-hidden="true" />
      )}
    </Button>
  )

  return isMobile ? button : <Tooltip label={label} side="bottom">{button}</Tooltip>
}

function SidebarRail({ className, ...props }: React.ComponentProps<"button">) {
  const { toggleSidebar } = useSidebar()

  return (
    <button
      type="button"
      data-sidebar="rail"
      data-slot="sidebar-rail"
      aria-label="Toggle Sidebar"
      tabIndex={-1}
      onClick={toggleSidebar}
      className={cn(
        "absolute inset-y-0 -right-4 z-20 hidden w-4 -translate-x-1/2 cursor-w-resize transition-[background-color,transform] duration-micro ease-linear after:absolute after:inset-y-0 after:left-1/2 after:w-[2px] hover:after:bg-sidebar-border in-data-[state=collapsed]:cursor-e-resize motion-reduce:transition-none sm:flex",
        className
      )}
      {...props}
    />
  )
}

// A div, not shadcn's <main>: the inset holds the app header too, and the
// header must stay outside the main landmark. AppShell marks up its content
// area as <main>.
function SidebarInset({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-inset"
      className={cn(
        "relative flex min-w-0 flex-1 flex-col bg-background",
        className
      )}
      {...props}
    />
  )
}

function SidebarHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-header"
      data-sidebar="header"
      className={cn("flex flex-col gap-2 p-2", className)}
      {...props}
    />
  )
}

function SidebarFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-footer"
      data-sidebar="footer"
      className={cn("flex flex-col gap-2 p-2", className)}
      {...props}
    />
  )
}

function SidebarContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-content"
      data-sidebar="content"
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-2 overflow-auto group-data-[collapsible=icon]:overflow-hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-group"
      data-sidebar="group"
      className={cn("relative flex w-full min-w-0 flex-col p-2", className)}
      {...props}
    />
  )
}

// The small heading over a group of links. It goes when the column is
// collapsed to icons, where there is no room for it.
function SidebarGroupLabel({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-group-label"
      data-sidebar="group-label"
      className={cn(
        "flex h-8 shrink-0 items-center px-2 text-xs font-medium tracking-wide text-sidebar-foreground/60 uppercase group-data-[collapsible=icon]:hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarGroupContent({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-group-content"
      data-sidebar="group-content"
      className={cn("w-full text-sm", className)}
      {...props}
    />
  )
}

function SidebarMenu({ className, ...props }: React.ComponentProps<"ul">) {
  return (
    <ul
      data-slot="sidebar-menu"
      data-sidebar="menu"
      className={cn("flex w-full min-w-0 flex-col gap-1", className)}
      {...props}
    />
  )
}

function SidebarMenuItem({ className, ...props }: React.ComponentProps<"li">) {
  return (
    <li
      data-slot="sidebar-menu-item"
      data-sidebar="menu-item"
      className={cn("relative", className)}
      {...props}
    />
  )
}

const SIDEBAR_MENU_BUTTON_SIZES = {
  default: "h-8 p-2 group-data-[collapsible=icon]:p-2!",
  // A row with an avatar and two lines of text: the account block.
  lg: "h-12 p-2 group-data-[collapsible=icon]:p-0!",
} as const

function SidebarMenuButton({
  asChild = false,
  isActive = false,
  size = "default",
  tooltip,
  className,
  ...props
}: React.ComponentProps<"button"> & {
  asChild?: boolean
  isActive?: boolean
  size?: keyof typeof SIDEBAR_MENU_BUTTON_SIZES
  /**
   * The row's name, shown beside it while the column is collapsed to icons.
   * It repeats the accessible name, so it is not announced again.
   */
  tooltip?: string
}) {
  const { isMobile, state } = useSidebar()
  const Comp = asChild ? Slot.Root : "button"

  const button = (
    <Comp
      data-slot="sidebar-menu-button"
      data-sidebar="menu-button"
      data-size={size}
      data-active={isActive}
      className={cn(
        "flex w-full items-center gap-2 overflow-hidden rounded-md text-left text-sm ring-sidebar-ring outline-hidden transition-[width,height,padding] duration-standard group-data-[collapsible=icon]:size-8! hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground motion-reduce:transition-none [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0",
        SIDEBAR_MENU_BUTTON_SIZES[size],
        className
      )}
      {...props}
    />
  )

  if (!tooltip || isMobile || state !== "collapsed") return button

  return (
    <Tooltip label={tooltip} side="right">
      {button}
    </Tooltip>
  )
}

export {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarGroupContent,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
}
