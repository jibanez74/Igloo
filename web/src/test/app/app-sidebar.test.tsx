import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const miniPlayer = vi.hoisted(() => ({ visible: false }));

vi.mock("@/hooks/useIsMiniPlayerVisible", () => ({
  useIsMiniPlayerVisible: () => miniPlayer.visible,
}));

function mockShellFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) =>
      requestURL(input) === "/api/auth/user"
        ? jsonResponse(authUser())
        : jsonResponse({ error: false, data: {} }),
    ),
  );
}

async function renderSidebar() {
  await renderRoute("/photos");
  await screen.findByRole("button", { name: "Logout" });
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
