export type SettingsTabId =
  | "general"
  | "account"
  | "libraries"
  | "playback"
  | "users";

export type SettingsTabDef = {
  id: SettingsTabId;
  path: string;
};

export type SettingsLayoutInput<T extends SettingsTabDef> = {
  isAdmin: boolean;
  pathname: string;
  tabs: readonly T[];
};

export type SettingsLayoutState<T extends SettingsTabDef> = {
  visibleTabs: T[];
  currentTab: SettingsTabId;
  defaultTabPath: string;
  redirectTo: string | null;
};
