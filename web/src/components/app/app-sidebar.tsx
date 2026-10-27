import { useState } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  Home,
  Music,
  Film,
  Tv,
  Settings,
  LogOut,
  type LucideIcon,
} from "lucide-react";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import { useSidebar } from "@/components/ui/sidebar-context";
import { logout } from "@/lib/api";
import {
  MINI_PLAYER_CLEARANCE_PADDING_CLASS,
  MOTION_MICRO_OPACITY_CLASS,
  MOVIES_INDEX_DEFAULT_SEARCH,
  SHOWS_INDEX_DEFAULT_SEARCH,
} from "@/lib/constants";
import { showError } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import BrandMark from "@/components/app/BrandMark";
import { apiErrorMessage } from "@/lib/is-api-failure";
import { useEndSession } from "@/hooks/useEndSession";
import { useIsMiniPlayerVisible } from "@/hooks/useIsMiniPlayerVisible";

type NavItem = {
  title: string;
  url: string;
  icon: LucideIcon;
  exact?: boolean;
};

// Library routes carry required search params; linking with their defaults
// keeps the sidebar href identical to what the loader would redirect to.
const NAV_DEFAULT_SEARCH: Partial<Record<string, object>> = {
  "/movies": MOVIES_INDEX_DEFAULT_SEARCH,
  "/tv-shows": SHOWS_INDEX_DEFAULT_SEARCH,
};

const navItems: NavItem[] = [
  { title: "Home", url: "/", icon: Home, exact: true },
  { title: "Movies", url: "/movies", icon: Film },
  { title: "TV Shows", url: "/tv-shows", icon: Tv },
  { title: "Music", url: "/music", icon: Music },
  { title: "Settings", url: "/settings", icon: Settings },
];

function SidebarItemContent({
  icon: Icon,
  title,
  active,
}: {
  icon: LucideIcon;
  title: string;
  active: boolean;
}) {
  return (
    <>
      <Icon
        className={active ? "text-primary" : "text-muted-foreground"}
        aria-hidden="true"
      />
      <span aria-hidden="true">{title}</span>
    </>
  );
}

export default function AppSidebar({
  ...props
}: React.ComponentProps<typeof Sidebar>) {
  const routerState = useRouterState();
  const endSession = useEndSession();
  const [loggingOut, setLoggingOut] = useState(false);
  const { isMobile, setOpenMobile } = useSidebar();
  // The fixed mini player spans the full width, so on desktop the sidebar
  // column stops above it and Logout stays clickable. The mobile sheet opens
  // over the bar and ignores this className.
  const isMiniPlayerVisible = useIsMiniPlayerVisible();
  const currentPath = routerState.location.pathname;

  // Close the mobile sidebar when navigating on a mobile screen
  const handleNavClick = () => {
    if (isMobile) {
      setOpenMobile(false);
    }
  };

  const isActive = (url: string, exact?: boolean) => {
    if (exact) {
      return currentPath === url;
    }

    return currentPath.startsWith(url);
  };

  const handleLogout = async () => {
    if (loggingOut) return;
    if (isMobile) {
      setOpenMobile(false);
    }
    setLoggingOut(true);

    const res = await logout();
    // A 401 means the session had already ended, which is all logout wants.
    if (res.error && res.status !== 401) {
      showError("Logout failed", apiErrorMessage(res, "Please try again."));
      setLoggingOut(false);
      return;
    }

    await endSession();
  };

  return (
    <Sidebar
      className={cn(
        "border-sidebar-border bg-sidebar **:data-[slot=sidebar-inner]:bg-sidebar",
        isMiniPlayerVisible && MINI_PLAYER_CLEARANCE_PADDING_CLASS,
      )}
      {...props}
    >
      {/* Header with Logo */}
      <SidebarHeader className="border-b border-sidebar-border p-4">
        <Link
          to="/"
          preload={false}
          onClick={handleNavClick}
          aria-label="Igloo – Home"
          className={cn(
            "flex items-center gap-3 hover:opacity-80",
            MOTION_MICRO_OPACITY_CLASS,
          )}
        >
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-lg shadow-primary/20">
            <BrandMark className="size-5" />
          </div>
          <span className="text-lg font-semibold text-sidebar-foreground group-data-[collapsible=icon]:hidden">
            Igloo
          </span>
        </Link>
      </SidebarHeader>

      {/* Main Navigation */}
      <SidebarContent
        role="navigation"
        aria-label="Main navigation"
        className="px-2 py-4"
      >
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {navItems.map(item => {
                const active = isActive(item.url, item.exact);

                return (
                  <SidebarMenuItem key={item.title}>
                    <SidebarMenuButton
                      asChild
                      isActive={active}
                      className={
                        active
                          ? "bg-sidebar-accent text-sidebar-accent-foreground hover:bg-sidebar-accent"
                          : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                      }
                    >
                      <Link
                        to={item.url}
                        preload={item.url === "/" ? false : undefined}
                        search={NAV_DEFAULT_SEARCH[item.url]}
                        onClick={handleNavClick}
                        aria-label={item.title}
                        aria-current={active ? "page" : undefined}
                      >
                        <SidebarItemContent
                          icon={item.icon}
                          title={item.title}
                          active={active}
                        />
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* Footer with Logout Button */}
      <SidebarFooter className="border-t border-sidebar-border p-2">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              onClick={handleLogout}
              aria-label="Logout"
              className="text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
            >
              <LogOut className="text-muted-foreground" aria-hidden="true" />
              <span aria-hidden="true">Logout</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      {/* Rail for collapse handle */}
      <SidebarRail />
    </Sidebar>
  );
}
