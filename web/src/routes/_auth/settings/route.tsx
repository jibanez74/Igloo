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
  LIBRARY_TAB_TRIGGER_CLASS,
  LIBRARY_TABS_LIST_CLASS,
} from "@/lib/constants";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";
import { authUserQueryOpts } from "@/lib/query-opts";
import { computeSettingsLayoutState } from "@/lib/settings-layout";
import { cn } from "@/lib/utils";
import { routeHead } from "@/lib/route-head";

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

// Subpages set only their own title, so they keep this description.
const SETTINGS_HEAD = routeHead(
  "Settings",
  "Configure your Igloo media center settings and preferences.",
);

export const Route = createFileRoute("/_auth/settings")({
  head: () => SETTINGS_HEAD,
  beforeLoad: async ({ context, location }) => {
    const authData = await context.queryClient.fetchQuery(
      authUserQueryOpts(),
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

  // Five tabs: two columns on a phone (the odd one spanning both), then from
  // `@md` five equal tabs filling the content width, and only from `@2xl` the
  // library pages' fit-to-content card. A 2+2+1 card beside empty space at
  // tablet width read as orphaned.
  const isCompactLayout = visibleTabs.length <= 2;
  const tabsListClassName = isCompactLayout
    ? cn(LIBRARY_TABS_LIST_CLASS, "grid-cols-2")
    : cn(
        LIBRARY_TABS_LIST_CLASS,
        "grid-cols-2 sm:w-full @md:grid-cols-5 @2xl:w-fit",
      );
  const tabsTriggerClassName = isCompactLayout
    ? LIBRARY_TAB_TRIGGER_CLASS
    : cn(
        LIBRARY_TAB_TRIGGER_CLASS,
        // Five labelled tabs fit a 28rem container only with the tighter
        // padding; the library padding returns with the fit-to-content card.
        "last:col-span-2 @md:px-1.5 @md:last:col-span-1 @2xl:px-4",
      );

  return (
    <div className="@container min-w-0">
      {/* Page header */}
      <header className="mb-6 sm:mb-7">
        <h1 className="flex items-center gap-3 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
          <Settings className="size-6 text-primary" aria-hidden="true" />
          <span>Settings</span>
        </h1>
        <p className="mt-2 max-w-2xl text-sm text-muted-foreground md:text-base">
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
                {/* The trigger's own gap spaces the icon, as on the library
                    tabs; a margin on top of it doubled the gap. */}
                <Icon
                  className="size-4 shrink-0 max-[360px]:hidden"
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
  );
}
