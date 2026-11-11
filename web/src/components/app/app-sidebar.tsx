import { useState, useSyncExternalStore } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useRouterState } from "@tanstack/react-router";
import {
  ChevronsUpDown,
  Home,
  Music,
  Film,
  Tv,
  Settings,
  LogOut,
  Moon,
  Sun,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
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
import { authUserFrom, authUserQueryOpts } from "@/lib/query-opts";
import { getActiveTheme, setTheme, subscribeTheme } from "@/lib/theme";
import { showError } from "@/lib/toast-helpers";
import { cn, getInitials } from "@/lib/utils";
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

// In reading order: Home, then the libraries under their own heading, then
// Settings at the foot of the column, next to the account it belongs with.
const HOME_ITEM: NavItem = { title: "Home", url: "/", icon: Home, exact: true };
const LIBRARY_ITEMS: NavItem[] = [
  { title: "Movies", url: "/movies", icon: Film },
  { title: "TV Shows", url: "/tv-shows", icon: Tv },
  { title: "Music", url: "/music", icon: Music },
];
const SETTINGS_ITEM: NavItem = {
  title: "Settings",
  url: "/settings",
  icon: Settings,
};

const LIBRARY_LABEL_ID = "sidebar-library-label";

const NAV_ITEM_CLASS = {
  active:
    "bg-sidebar-accent text-sidebar-accent-foreground hover:bg-sidebar-accent",
  idle: "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
} as const;

export default function AppSidebar({
  ...props
}: React.ComponentProps<typeof Sidebar>) {
  const routerState = useRouterState();
  const endSession = useEndSession();
  const [loggingOut, setLoggingOut] = useState(false);
  const { isMobile, setOpenMobile, state } = useSidebar();
  const { data: userData } = useQuery(authUserQueryOpts());
  const user = authUserFrom(userData);
  const theme = useSyncExternalStore(subscribeTheme, getActiveTheme);
  const nextTheme = theme === "dark" ? "light" : "dark";
  // The fixed mini player spans the full width, so on desktop the sidebar
  // column stops above it and the account block stays clickable. The mobile
  // sheet opens over the bar and ignores this className.
  const isMiniPlayerVisible = useIsMiniPlayerVisible();
  const currentPath = routerState.location.pathname;
  const collapsed = !isMobile && state === "collapsed";

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

  const renderNavItem = (item: NavItem) => {
    const active = isActive(item.url, item.exact);
    const Icon = item.icon;

    return (
      <SidebarMenuItem key={item.title}>
        <SidebarMenuButton
          asChild
          isActive={active}
          tooltip={item.title}
          className={active ? NAV_ITEM_CLASS.active : NAV_ITEM_CLASS.idle}
        >
          <Link
            to={item.url}
            preload={item.url === "/" ? false : undefined}
            search={NAV_DEFAULT_SEARCH[item.url]}
            onClick={handleNavClick}
            aria-label={item.title}
            aria-current={active ? "page" : undefined}
          >
            <Icon
              className={active ? "text-primary" : "text-muted-foreground"}
              aria-hidden="true"
            />
            <span aria-hidden="true">{item.title}</span>
          </Link>
        </SidebarMenuButton>
      </SidebarMenuItem>
    );
  };

  const accountName = user?.name ?? "Account";
  const accountRole = user ? (user.is_admin ? "Administrator" : "Member") : "";

  return (
    <Sidebar
      className={cn(
        "border-sidebar-border bg-sidebar **:data-[slot=sidebar-inner]:bg-sidebar",
        isMiniPlayerVisible && MINI_PLAYER_CLEARANCE_PADDING_CLASS,
      )}
      {...props}
    >
      {/* The header is as tall as the app header beside it, so the two
          bottom borders meet in one line. */}
      <SidebarHeader className="h-14 shrink-0 flex-row items-center border-b border-sidebar-border px-4 py-0 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0">
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
          <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-glow">
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
        className="gap-0 px-2 py-3"
      >
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>{renderNavItem(HOME_ITEM)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel id={LIBRARY_LABEL_ID}>Library</SidebarGroupLabel>
          <SidebarGroupContent>
            {/* Named by its heading when that shows, and by itself when the
                column is collapsed and the heading is gone. */}
            <SidebarMenu
              aria-labelledby={collapsed ? undefined : LIBRARY_LABEL_ID}
              aria-label={collapsed ? "Library" : undefined}
            >
              {LIBRARY_ITEMS.map(renderNavItem)}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup className="mt-auto">
          <SidebarGroupContent>
            <SidebarMenu>{renderNavItem(SETTINGS_ITEM)}</SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* Who is signed in, and what belongs to them. */}
      <SidebarFooter className="border-t border-sidebar-border p-2">
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  tooltip={accountName}
                  aria-label={`Account: ${accountName}`}
                  className="text-sidebar-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[state=open]:bg-sidebar-accent"
                >
                  <Avatar className="size-8 rounded-lg">
                    {user?.avatar ? (
                      <AvatarImage src={user.avatar} alt="" />
                    ) : null}
                    <AvatarFallback className="rounded-lg bg-primary/15 text-xs font-semibold text-primary">
                      {user ? (
                        getInitials(user.name)
                      ) : (
                        <UserRound className="size-4" aria-hidden="true" />
                      )}
                    </AvatarFallback>
                  </Avatar>
                  <span
                    aria-hidden="true"
                    className="grid min-w-0 flex-1 leading-tight group-data-[collapsible=icon]:hidden"
                  >
                    <span className="truncate text-sm font-medium">
                      {accountName}
                    </span>
                    {accountRole ? (
                      <span className="truncate text-xs text-muted-foreground">
                        {accountRole}
                      </span>
                    ) : null}
                  </span>
                  <ChevronsUpDown
                    className="ml-auto text-muted-foreground group-data-[collapsible=icon]:hidden"
                    aria-hidden="true"
                  />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                side={collapsed ? "right" : "top"}
                align={collapsed ? "end" : "start"}
                className="w-(--radix-dropdown-menu-trigger-width) min-w-56"
              >
                {user ? (
                  <>
                    <DropdownMenuLabel className="grid leading-tight">
                      <span className="truncate">{user.name}</span>
                      <span className="truncate text-xs font-normal text-muted-foreground">
                        {user.email}
                      </span>
                    </DropdownMenuLabel>
                    <DropdownMenuSeparator />
                  </>
                ) : null}
                <DropdownMenuItem asChild>
                  <Link to="/settings/account" onClick={handleNavClick}>
                    <UserRound aria-hidden="true" />
                    Account settings
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={() => setTheme(nextTheme)}>
                  {nextTheme === "light" ? (
                    <Sun aria-hidden="true" />
                  ) : (
                    <Moon aria-hidden="true" />
                  )}
                  {nextTheme === "light"
                    ? "Switch to light theme"
                    : "Switch to dark theme"}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  disabled={loggingOut}
                  onSelect={() => void handleLogout()}
                >
                  <LogOut aria-hidden="true" />
                  Logout
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      {/* Rail for collapse handle */}
      <SidebarRail />
    </Sidebar>
  );
}
