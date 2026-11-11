import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
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

describe("app sidebar navigation", () => {
  beforeEach(() => {
    miniPlayer.visible = false;
    localStorage.clear();
    mockShellFetch();
  });

  it("puts the libraries under a Library heading that names their list", async () => {
    await renderSidebar();

    const navigation = screen.getByRole("navigation", { name: "Main navigation" });
    const library = within(navigation).getByRole("list", { name: "Library" });
    expect(within(library).getAllByRole("link").map(link => link.textContent)).toEqual([
      "Movies", "TV Shows", "Music",
    ]);
    // Home leads and Settings closes the column, each outside that group.
    expect(within(library).queryByRole("link", { name: "Home" })).toBeNull();
    expect(within(library).queryByRole("link", { name: "Settings" })).toBeNull();
  });

  it("collapses to icons from the header, says so, and remembers it", async () => {
    const user = userEvent.setup();
    const sidebar = await renderSidebar();
    const column = sidebar?.closest('[data-slot="sidebar"]');
    expect(column).toHaveAttribute("data-state", "expanded");

    const toggle = screen.getByRole("button", { name: "Collapse sidebar" });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    await user.click(toggle);

    expect(column).toHaveAttribute("data-state", "collapsed");
    expect(
      screen.getByRole("button", { name: "Expand sidebar" }),
    ).toHaveAttribute("aria-expanded", "false");
    expect(localStorage.getItem("igloo-sidebar")).toBe("collapsed");
    // The heading is gone when collapsed; the list keeps its name.
    expect(screen.getByRole("list", { name: "Library" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Expand sidebar" }));
    expect(column).toHaveAttribute("data-state", "expanded");
    expect(localStorage.getItem("igloo-sidebar")).toBe("expanded");
  });

  it("opens collapsed for a reader who left it collapsed", async () => {
    localStorage.setItem("igloo-sidebar", "collapsed");

    const sidebar = await renderSidebar();

    expect(sidebar?.closest('[data-slot="sidebar"]')).toHaveAttribute(
      "data-state",
      "collapsed",
    );
  });

  it("names a collapsed icon in a tooltip on keyboard focus, and not when expanded", async () => {
    const user = userEvent.setup();
    await renderSidebar();
    const navigation = screen.getByRole("navigation", { name: "Main navigation" });

    within(navigation).getByRole("link", { name: "Movies" }).focus();
    expect(screen.queryByRole("tooltip")).toBeNull();

    await user.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    // Collapsing wraps the link in its tooltip, so it is looked up again.
    const movies = within(navigation).getByRole("link", { name: "Movies" });
    act(() => movies.focus());

    expect(await screen.findByRole("tooltip")).toHaveTextContent("Movies");
    // It repeats the link's name, so it must not be announced again.
    expect(movies).not.toHaveAttribute("aria-describedby");
  });
});

describe("app sidebar account block", () => {
  beforeEach(() => {
    miniPlayer.visible = false;
    localStorage.clear();
    document.documentElement.className = "dark";
    mockShellFetch();
  });

  it("shows who is signed in and their role", async () => {
    await renderSidebar();

    const account = screen.getByRole("button", { name: "Account: Test User" });
    expect(within(account).getByText("Test User")).toBeInTheDocument();
    expect(within(account).getByText("Member")).toBeInTheDocument();
    // No uploaded picture: the initials stand in.
    expect(within(account).getByText("TU")).toBeInTheDocument();
  });

  it("opens a menu with the account's email, its settings, the theme and Logout", async () => {
    const user = userEvent.setup();
    await renderSidebar();

    await user.click(screen.getByRole("button", { name: "Account: Test User" }));

    const menu = await screen.findByRole("menu");
    expect(within(menu).getByText("test@example.com")).toBeInTheDocument();
    expect(
      within(menu).getAllByRole("menuitem").map(item => item.textContent),
    ).toEqual(["Account settings", "Switch to light theme", "Logout"]);
    expect(
      within(menu).getByRole("menuitem", { name: "Account settings" }),
    ).toHaveAttribute("href", "/settings/account");
  });

  it("switches the theme from the menu", async () => {
    const user = userEvent.setup();
    await renderSidebar();

    await user.click(screen.getByRole("button", { name: "Account: Test User" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Switch to light theme" }),
    );

    expect(document.documentElement).not.toHaveClass("dark");
    expect(localStorage.getItem("igloo-theme")).toBe("light");
  });

  it("signs out from the menu", async () => {
    const user = userEvent.setup();
    await renderSidebar();
    const fetchMock = vi.mocked(fetch);

    await user.click(screen.getByRole("button", { name: "Account: Test User" }));
    await user.click(await screen.findByRole("menuitem", { name: "Logout" }));

    const logoutCall = fetchMock.mock.calls.find(([input, init]) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
      return url.includes("/api/auth/logout") && init?.method === "DELETE";
    });
    expect(logoutCall).toBeDefined();
  });
});
