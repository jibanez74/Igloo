import { describe, it, expect } from "vitest";
import { computeSettingsLayoutState } from "@/lib/settings-layout";

const TABS = [
  { id: "general" as const, path: "/settings" },
  { id: "account" as const, path: "/settings/account" },
  { id: "libraries" as const, path: "/settings/libraries" },
  { id: "playback" as const, path: "/settings/playback" },
  { id: "users" as const, path: "/settings/users" },
];

describe("computeSettingsLayoutState — admin", () => {
  it("exposes all 5 tabs and lands on general at /settings", () => {
    const state = computeSettingsLayoutState({
      isAdmin: true,
      pathname: "/settings",
      tabs: TABS,
    });
    expect(state.visibleTabs.map(t => t.id)).toEqual([
      "general",
      "account",
      "libraries",
      "playback",
      "users",
    ]);
    expect(state.currentTab).toBe("general");
    expect(state.redirectTo).toBeNull();
    expect(state.defaultTabPath).toBe("/settings");
  });

  it("respects admin-only tab in URL without redirecting", () => {
    const state = computeSettingsLayoutState({
      isAdmin: true,
      pathname: "/settings/users",
      tabs: TABS,
    });
    expect(state.currentTab).toBe("users");
    expect(state.redirectTo).toBeNull();
  });
});

describe("computeSettingsLayoutState — non-admin", () => {
  it("filters admin-only tabs out", () => {
    const state = computeSettingsLayoutState({
      isAdmin: false,
      pathname: "/settings",
      tabs: TABS,
    });
    expect(state.visibleTabs.map(t => t.id)).toEqual(["account", "playback"]);
    expect(state.defaultTabPath).toBe("/settings/account");
    expect(state.currentTab).toBe("account");
    expect(state.redirectTo).toBe("/settings/account");
  });

  it("redirects to /settings/account when URL targets an admin-only tab", () => {
    const state = computeSettingsLayoutState({
      isAdmin: false,
      pathname: "/settings/general",
      tabs: TABS,
    });
    expect(state.currentTab).toBe("account");
    expect(state.redirectTo).toBe("/settings/account");
  });

  it("redirects /settings/users (admin-only) to /settings/account", () => {
    const state = computeSettingsLayoutState({
      isAdmin: false,
      pathname: "/settings/users",
      tabs: TABS,
    });
    expect(state.redirectTo).toBe("/settings/account");
  });

  it("does not redirect when already on a visible tab", () => {
    const state = computeSettingsLayoutState({
      isAdmin: false,
      pathname: "/settings/playback",
      tabs: TABS,
    });
    expect(state.currentTab).toBe("playback");
    expect(state.redirectTo).toBeNull();
  });

  it("does not redirect when already on /settings/account", () => {
    const state = computeSettingsLayoutState({
      isAdmin: false,
      pathname: "/settings/account",
      tabs: TABS,
    });
    expect(state.currentTab).toBe("account");
    expect(state.redirectTo).toBeNull();
  });
});
