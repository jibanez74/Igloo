import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import {
  Home,
  Music,
  Film,
  Tv,
  Image,
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
  MOTION_MICRO_OPACITY_CLASS,
  MOVIES_INDEX_DEFAULT_SEARCH,
} from "@/lib/constants";
import { showError } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";

type NavItem = {
  title: string;
  url: string;
  icon: LucideIcon;
  exact?: boolean;
};

const navItems: NavItem[] = [
  { title: "Home", url: "/", icon: Home, exact: true },
  { title: "Movies", url: "/movies", icon: Film },
  { title: "TV Shows", url: "/tv-shows", icon: Tv },
  { title: "Music", url: "/music", icon: Music },
  { title: "Photos", url: "/photos", icon: Image },
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
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { isMobile, setOpenMobile } = useSidebar();
  const currentPath = routerState.location.pathname;

  // Close the mobile sidebar when navigating
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
    if (isMobile) {
      setOpenMobile(false);
    }

    const res = await logout();
    if (res.error) {
      showError("Logout failed", res.message || "Please try again.");
      return;
    }

    queryClient.removeQueries();
    navigate({ to: "/login", replace: true });
  };

  return (
    <Sidebar
      collapsible="icon"
      className="border-sidebar-border bg-sidebar **:data-[slot=sidebar-inner]:bg-sidebar"
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
            <span className="text-lg font-bold">I</span>
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
                        search={
                          item.url === "/movies"
                            ? MOVIES_INDEX_DEFAULT_SEARCH
                            : undefined
                        }
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
