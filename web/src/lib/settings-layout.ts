import type {
  SettingsLayoutInput,
  SettingsLayoutState,
  SettingsTabDef,
  SettingsTabId,
} from "@/types";

export const SETTINGS_TAB_PATHS: Record<SettingsTabId, string> = {
  general: "/settings",
  account: "/settings/account",
  libraries: "/settings/libraries",
  playback: "/settings/playback",
  users: "/settings/users",
};

export const ADMIN_ONLY_SETTINGS_TABS: ReadonlySet<SettingsTabId> = new Set([
  "general",
  "libraries",
  "users",
]);

export function computeSettingsLayoutState<T extends SettingsTabDef>(
  input: SettingsLayoutInput<T>,
): SettingsLayoutState<T> {
  const { isAdmin, pathname, tabs } = input;

  const visibleTabs = tabs.filter(
    tab => isAdmin || !ADMIN_ONLY_SETTINGS_TABS.has(tab.id),
  );

  const defaultTab: SettingsTabId = isAdmin ? "general" : "account";
  const defaultTabPath =
    tabs.find(t => t.id === defaultTab)?.path ?? "/settings";

  const pathParts = pathname.split("/").filter(Boolean);
  const urlTabId =
    pathParts.length === 1 && pathParts[0] === "settings"
      ? "general"
      : (pathParts[1] as SettingsTabId | undefined);

  const urlTabAllowed =
    urlTabId !== undefined && visibleTabs.some(tab => tab.id === urlTabId);
  const currentTab: SettingsTabId = urlTabAllowed
    ? (urlTabId as SettingsTabId)
    : defaultTab;

  const redirectTo =
    !urlTabAllowed && pathname !== defaultTabPath ? defaultTabPath : null;

  return { visibleTabs, currentTab, defaultTabPath, redirectTo };
}
