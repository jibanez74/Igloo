import { Link, useRouterState } from "@tanstack/react-router";
import {
  Home,
  Music,
  Film,
  Tv,
  Image,
  Settings,
  type LucideIcon,
} from "lucide-react";

import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";

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

export default function AppSidebar({
  ...props
}: React.ComponentProps<typeof Sidebar>) {
  const routerState = useRouterState();
  const currentPath = routerState.location.pathname;

  const isActive = (url: string, exact?: boolean) => {
    if (exact) {
      return currentPath === url;
    }
    return currentPath.startsWith(url);
  };

  return (
    <Sidebar
      collapsible="icon"
      className="border-slate-800/50 bg-slate-900 **:data-[slot=sidebar-inner]:bg-slate-900"
      {...props}
    >
      {/* Header with Logo */}
      <SidebarHeader className="border-b border-slate-800/50 p-4">
        <Link
          to="/"
          className="flex items-center gap-3 transition-opacity hover:opacity-80"
        >
          <div className="flex size-8 items-center justify-center rounded-lg bg-amber-500 text-slate-900 shadow-lg shadow-amber-500/20">
            <span className="text-lg font-bold">I</span>
          </div>
          <span className="text-lg font-semibold text-white group-data-[collapsible=icon]:hidden">
            Igloo
          </span>
        </Link>
      </SidebarHeader>

      {/* Main Navigation */}
      <SidebarContent className="px-2 py-4">
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
                      tooltip={item.title}
                      className={
                        active
                          ? "bg-slate-800 text-white hover:bg-slate-700"
                          : "text-slate-300 hover:bg-slate-800/50 hover:text-white"
                      }
                    >
                      <Link to={item.url}>
                        <item.icon
                          className={
                            active ? "text-amber-400" : "text-slate-400"
                          }
                        />
                        <span>{item.title}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* Rail for collapse handle */}
      <SidebarRail />
    </Sidebar>
  );
}
