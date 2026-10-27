import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUserData } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const savedSettings = {
  tmdb_key: null,
  spotify_client_id: null,
  spotify_client_secret: null,
  enable_apple_lookups: true,
  enable_watcher: false,
  download_images: true,
  static_dir: "/var/lib/igloo/static",
  transcode_dir: "/var/lib/igloo/transcode",
};

function mockGeneralSettings() {
  const saves: Record<string, unknown>[] = [];
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: { user: authUserData({ is_admin: true }) },
      });
    }
    if (url === "/api/settings/general" && init?.method === "PUT") {
      const body = JSON.parse(String(init.body)) as Record<string, unknown>;
      saves.push(body);
      return jsonResponse({
        error: false,
        message: "Settings updated",
        data: {
          settings: { ...savedSettings, ...body, tmdb_key: null },
          restart_required: false,
        },
      });
    }
    if (url === "/api/settings/general") {
      return jsonResponse({ error: false, data: { settings: savedSettings } });
    }
    return jsonResponse({ error: true, message: `Unexpected ${url}` }, 500);
  });
  vi.stubGlobal("fetch", fetchMock);
  return saves;
}

describe("general settings: download images", () => {
  it("asks before a save that turns downloads off, since it deletes them", async () => {
    const saves = mockGeneralSettings();
    const user = userEvent.setup();
    await renderRoute("/settings");

    const downloads = await screen.findByRole("switch", {
      name: "Download images",
    });
    expect(downloads).toHaveAccessibleDescription(/deletes the saved images/);
    await user.click(downloads);
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    const dialog = await screen.findByRole("alertdialog", {
      name: "Delete downloaded images?",
    });
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(saves).toHaveLength(0);
    expect(downloads).toHaveAttribute("aria-checked", "false");
    // Focus goes back to what opened the dialog.
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Save Settings" }),
      ).toHaveFocus();
    });

    await user.click(screen.getByRole("button", { name: "Save Settings" }));
    await user.click(
      within(
        await screen.findByRole("alertdialog", {
          name: "Delete downloaded images?",
        }),
      ).getByRole("button", { name: "Delete images" }),
    );

    await waitFor(() => {
      expect(saves).toHaveLength(1);
    });
    expect(saves[0]).toMatchObject({ download_images: false });
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    // The saved form disables Save, so focus lands on the switch instead.
    expect(screen.getByRole("button", { name: "Save Settings" })).toBeDisabled();
    await waitFor(() => {
      expect(downloads).toHaveFocus();
    });
  });

  it("saves other changes without asking", async () => {
    const saves = mockGeneralSettings();
    const user = userEvent.setup();
    await renderRoute("/settings");

    await user.click(
      await screen.findByRole("switch", { name: "Library watcher" }),
    );
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    await waitFor(() => {
      expect(saves).toHaveLength(1);
    });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(saves[0]).toMatchObject({ download_images: true });
  });
});
