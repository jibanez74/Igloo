import {
  createFileRoute,
  Outlet,
  redirect,
  useLocation,
} from "@tanstack/react-router";
import { useSuspenseQuery } from "@tanstack/react-query";
import { Settings, User, Sliders, Library, Play, Users } from "lucide-react";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import {
  CONTENT_FADE_ENTER_CLASS,
  CONTENT_FADE_EXIT_CLASS,
  CONTENT_FADE_TRANSITION_MS,
} from "@/lib/constants";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { authUserGuardQueryOpts, authUserQueryOpts } from "@/lib/query-opts";
import { computeSettingsLayoutState } from "@/lib/settings-layout";
import { cn } from "@/lib/utils";

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

export const Route = createFileRoute("/_auth/settings")({
  beforeLoad: async ({ context, location }) => {
    const authData = await context.queryClient.fetchQuery(
      authUserGuardQueryOpts(),
    );
    if (authData.error) {
      throw redirect({
        to: "/login",
        search: { redirect: location.href },
      });
    }

    const { redirectTo } = computeSettingsLayoutState({
      isAdmin: authData.data.user.is_admin,
      pathname: location.pathname,
      tabs: SETTINGS_TABS,
    });
    if (redirectTo) {
      throw redirect({ to: redirectTo, replace: true });
    }
  },
  component: SettingsLayout,
});

function SettingsLayout() {
  const navigate = Route.useNavigate();
  const location = useLocation();
  const { data: authData } = useSuspenseQuery(authUserQueryOpts());
  const isAdmin = authData.data?.user.is_admin ?? false;
  const {
    isExiting,
    runTransition,
    usesContentAnimation,
  } = useContentFadeTransition(CONTENT_FADE_TRANSITION_MS);

  const { visibleTabs, currentTab } = computeSettingsLayoutState({
    isAdmin,
    pathname: location.pathname,
    tabs: SETTINGS_TABS,
  });

  const handleTabChange = (newTab: string) => {
    const tab = visibleTabs.find(t => t.id === newTab);
    if (!tab || tab.id === currentTab) {
      return;
    }

    runTransition({
      shouldAnimate: true,
      onTransition: () =>
        navigate({
          to: tab.path,
          replace: true,
        }),
    });
  };

  const isCompactLayout = visibleTabs.length <= 2;
  const tabsListClassName = isCompactLayout
    ? "grid! h-auto w-full max-w-full grid-cols-2 gap-1 border border-slate-700/50 bg-slate-800/50 p-1 sm:w-fit sm:max-w-none"
    : "grid! h-auto w-full max-w-full grid-cols-2 gap-1 border border-slate-700/50 bg-slate-800/50 p-1 min-[520px]:grid-cols-3 sm:w-fit sm:max-w-none sm:grid-cols-5";
  const tabsTriggerClassName = isCompactLayout
    ? "min-h-10 min-w-0 p-2 text-sm text-slate-400 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 sm:px-4"
    : "min-h-10 min-w-0 p-2 text-sm text-slate-400 last:col-span-2 hover:text-white data-[state=active]:bg-amber-500 data-[state=active]:text-slate-900 data-[state=active]:shadow-lg data-[state=active]:shadow-amber-500/20 min-[520px]:last:col-span-2 sm:px-4 sm:last:col-span-1";

  return (
    <>
      {/* React 19 Document Metadata */}
      <title>Settings - Igloo</title>
      <meta
        name="description"
        content="Configure your Igloo media center settings and preferences."
      />

      <div className="min-w-0">
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
          <TabsList className={tabsListClassName}>
            {visibleTabs.map(tab => {
              const Icon = tab.icon;
              return (
                <TabsTrigger
                  key={tab.id}
                  value={tab.id}
                  className={tabsTriggerClassName}
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

          <TabsContent value={currentTab} className="mt-6">
            <div
              key={location.pathname}
              className={cn(
                usesContentAnimation &&
                  (isExiting ? CONTENT_FADE_EXIT_CLASS : CONTENT_FADE_ENTER_CLASS),
              )}
            >
              <Outlet />
            </div>
          </TabsContent>
        </Tabs>
      </div>
    </>
  );
}
