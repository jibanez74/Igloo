import { screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { homeShellResponse } from "../helpers/home";
import { renderRoute } from "../helpers/render-route";

const miniPlayer = vi.hoisted(() => ({ visible: false }));

vi.mock("@/hooks/useIsMiniPlayerVisible", () => ({
  useIsMiniPlayerVisible: () => miniPlayer.visible,
}));

function mockShellFetch() {
  vi.stubGlobal("fetch", vi.fn(homeShellResponse));
}

async function renderSidebar() {
  await renderRoute("/");
  await screen.findByRole("heading", { name: "Welcome to Igloo" });
  const navigation = screen.getByRole("navigation", { name: "Main navigation" });
  expect(within(navigation).getAllByRole("link").map(link => link.textContent)).toEqual([
    "Home", "Movies", "TV Shows", "Music", "Settings",
  ]);
  expect(within(navigation).getByRole("link", { name: "Home" })).toHaveAttribute(
    "aria-current", "page",
  );
  return document.querySelector('[data-slot="sidebar-container"]');
}

// The mini player is a full-width fixed bar, and the desktop sidebar is a
// full-height fixed column whose footer holds Logout: the column must end
// above the bar or the bar covers the button.
describe("app sidebar beside the mini player", () => {
  it("ends the desktop column above the bar while it shows", async () => {
    miniPlayer.visible = true;
    mockShellFetch();

    const sidebar = await renderSidebar();

    expect(sidebar).toHaveClass("pb-28", "sm:pb-24");
  });

  it("runs the column to the bottom without the bar", async () => {
    miniPlayer.visible = false;
    mockShellFetch();

    const sidebar = await renderSidebar();

    expect(sidebar).not.toBeNull();
    expect(sidebar).not.toHaveClass("pb-28");
  });
});
