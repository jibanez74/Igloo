import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { Settings, User, Sliders, Library, Play, Users } from "lucide-react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

export const Route = createFileRoute("/_admin/settings")({
  component: SettingsLayout,
});

const SETTINGS_TABS = [
  { id: "general", label: "General", icon: Sliders, path: "/settings" },
  { id: "account", label: "Account", icon: User, path: "/settings/account" },
  {
    id: "libraries",
    label: "Libraries",
    icon: Library,
    path: "/settings/libraries",
  },
  { id: "playback", label: "Playback", icon: Play, path: "/settings/playback" },
  { id: "users", label: "Users", icon: Users, path: "/settings/users" },
] as const;

type TabId = (typeof SETTINGS_TABS)[number]["id"];

function SettingsLayout() {
  const navigate = Route.useNavigate();
  const location = useLocation();

  const getCurrentTab = (): TabId => {
    const pathParts = location.pathname.split("/").filter(Boolean);

    // If pathname is exactly "/settings", it's the index route (general)
    if (pathParts.length === 1 && pathParts[0] === "settings") {
      return "general";
    }

    // Otherwise, get the tab name from the second path segment
    const tabId = pathParts[1] as TabId | undefined;
    return tabId && SETTINGS_TABS.some(tab => tab.id === tabId)
      ? tabId
      : "general";
  };

  const currentTab = getCurrentTab();

  // Handle tab change - navigate to the appropriate route
  const handleTabChange = (newTab: string) => {
    const tab = SETTINGS_TABS.find(t => t.id === newTab);
    if (tab) {
      navigate({
        to: tab.path,
        replace: true,
      });
    }
  };

  return (
    <>
      {/* React 19 Document Metadata */}
      <title>Settings - Igloo</title>
      <meta
        name="description"
        content="Configure your Igloo media center settings and preferences."
      />

      <div className="min-w-0 animate-in duration-300 fade-in">
        {/* Page header */}
        <header className="mb-6 sm:mb-7">
          <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-white md:text-4xl">
            <Settings className="size-6 text-amber-400" aria-hidden="true" />
            <span>Settings</span>
          </h1>
          <p className="mt-2 max-w-2xl text-sm text-slate-400 md:text-base">
            Manage application settings, your account, and users
          </p>
        </header>

        {/* Tabs */}
        <Tabs value={currentTab} onValueChange={handleTabChange}>
          <TabsList className="grid! h-auto w-full max-w-full grid-cols-2 gap-1 border border-slate-700/50 bg-slate-800/50 p-1 min-[520px]:grid-cols-3 sm:w-fit sm:max-w-none sm:grid-cols-5">
            {SETTINGS_TABS.map(tab => {
              const Icon = tab.icon;
              return (
                <TabsTrigger
                  key={tab.id}
                  value={tab.id}
                  className="min-h-10 min-w-0 p-2 text-sm text-slate-400 last:col-span-2 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 min-[520px]:last:col-span-2 sm:px-4 sm:last:col-span-1"
                >
                  <Icon
                    className="mr-1.5 size-4 shrink-0 max-[360px]:hidden sm:mr-2"
                    aria-hidden="true"
                  />
                  {tab.label}
                </TabsTrigger>
              );
            })}
          </TabsList>

          {/* Child route content */}
          <div className="mt-6">
            <Outlet />
          </div>
        </Tabs>
      </div>
    </>
  );
}
