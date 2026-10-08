import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerNowPlayingContext } from "@/context/AudioPlayerContext";
import {
  GENERAL_SETTINGS_KEY,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
  SETTINGS_KEY,
} from "@/lib/constants";
import type { AudioPlayerNowPlaying } from "@/types";
import { runContentFadeTransitionTimeout } from "../helpers/content-fade-transition";
import { jsonResponse, requestURL } from "../helpers/api";
import { restoreMatchMedia } from "../helpers/dom";
import { authUserData, playbackSettings } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const originalStartViewTransition = (document as Document & {
  startViewTransition?: unknown;
}).startViewTransition;

function authUser() {
  return authUserData({
    name: "Settings User",
    email: "settings@example.com",
    is_admin: true,
  });
}

function generalSettings() {
  return {
    tmdb_key: null,
    immich_base_url: null,
    immich_api_key: null,
    jellyfin_base_url: null,
    jellyfin_api_key: null,
    spotify_client_id: null,
    spotify_client_secret: null,
    enable_watcher: false,
    download_images: true,
    static_dir: "/var/lib/igloo/static",
    transcode_dir: "/var/lib/igloo/transcode",
  };
}

function librarySettings() {
  return {
    movies_dir: "/media/movies",
    shows_dir: "/media/shows",
    music_dir: "/media/music",
  };
}

function mockSettingsFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: authUser(),
        },
      });
    }

    if (url === "/api/settings/general") {
      return jsonResponse({
        error: false,
        data: {
          settings: generalSettings(),
        },
      });
    }

    if (url === "/api/settings/playback") {
      return jsonResponse({
        error: false,
        data: {
          settings: playbackSettings({ server_upload_mbps: 20 }),
        },
      });
    }

    if (url === "/api/settings") {
      return jsonResponse({
        error: false,
        data: librarySettings(),
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
}

async function renderSettingsRoute(
  initialEntry: string,
  nowPlaying: AudioPlayerNowPlaying | null = null,
) {
  mockSettingsFetch();

  const { queryClient, router } = await renderRoute(initialEntry, {
    wrapper: children => (
      <AudioPlayerNowPlayingContext.Provider value={nowPlaying}>
        {children}
      </AudioPlayerNowPlayingContext.Provider>
    ),
  });

  return { queryClient, router };
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  restoreMatchMedia();

  if (originalStartViewTransition === undefined) {
    Reflect.deleteProperty(document, "startViewTransition");
  } else {
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: originalStartViewTransition,
    });
  }
});

describe("settings route tab transitions", () => {
  it("delays tab route changes without using native view transitions", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    const startViewTransition = vi.fn((callback: () => void) => {
      callback();
      return {
        finished: Promise.resolve(),
        ready: Promise.resolve(),
        updateCallbackDone: Promise.resolve(),
        skipTransition: vi.fn(),
      };
    });

    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: startViewTransition,
    });

    await renderSettingsRoute("/settings");

    expect(await screen.findByText("Application Behavior")).toBeInTheDocument();

    // Warm the route module so navigation renders without a cold dynamic
    // import racing the waitFor timeout on slow CI runners.
    await import("@/routes/_auth/settings/account");

    await user.click(screen.getByRole("tab", { name: "Account" }));

    expect(screen.getByText("Application Behavior")).toBeInTheDocument();
    expect(screen.queryByText("Profile Information")).not.toBeInTheDocument();
    expect(startViewTransition).not.toHaveBeenCalled();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(() => {
      expect(screen.getByText("Profile Information")).toBeInTheDocument();
    });
    expect(startViewTransition).not.toHaveBeenCalled();
  }, 10_000);

  it("uses shared motion contracts for settings surfaces and switches", async () => {
    await renderSettingsRoute("/settings");

    const title = await screen.findByText("Application Behavior");
    expect(title.closest('[data-slot="card"]')).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );

    const savePanel =
      screen.getByText("General settings").parentElement?.parentElement;
    expect(savePanel).toHaveClass(...MOTION_SETTINGS_SURFACE_CLASS.split(" "));
    // Opaque and in flow while clean; the floating, translucent bar used to
    // let the form read through it (design-system §3.7).
    expect(savePanel).toHaveClass("bg-card");
    expect(savePanel).not.toHaveClass("sticky", "backdrop-blur-md");
    expect(savePanel).toHaveTextContent("No unsaved changes");
    expect(screen.getByRole("button", { name: "Reset" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save Settings" })).toBeDisabled();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Static directory"), "x");
    expect(savePanel).toHaveClass("sticky", "bottom-4");
    expect(savePanel).toHaveTextContent(
      "Saved settings are used by the backend on future requests.",
    );
    expect(screen.getByRole("button", { name: "Reset" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Save Settings" })).toBeEnabled();

    const switchControl = screen.getByRole("switch", { name: "Library watcher" });
    expect(switchControl).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );
    expect(switchControl.firstElementChild).toHaveClass(
      ...MOTION_CONTROL_THUMB_TRANSFORM_CLASS.split(" "),
    );
  });

  it("keeps sticky actions above the minimized audio player", async () => {
    await renderSettingsRoute("/settings", {
      currentTrackId: 1,
      isPlaying: true,
      isExpanded: false,
    });

    const savePanel =
      (await screen.findByText("General settings")).parentElement?.parentElement;
    // The offset only matters once the bar sticks, which it does while dirty.
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Static directory"), "x");
    expect(savePanel).toHaveClass("sticky", "bottom-28", "sm:bottom-24");
    expect(savePanel).not.toHaveClass("bottom-4");
  });
});

describe("settings form query updates", () => {
  it("updates a clean general settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings");

    expect(await screen.findByLabelText("Static directory")).toHaveValue(
      "/var/lib/igloo/static",
    );

    await act(async () => {
      queryClient.setQueryData([GENERAL_SETTINGS_KEY], {
        error: false,
        data: {
          settings: {
            ...generalSettings(),
            static_dir: "/srv/igloo/static",
            transcode_dir: "/srv/igloo/transcode",
          },
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Static directory")).toHaveValue(
        "/srv/igloo/static",
      );
    });
  });

  it("preserves dirty general settings edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings");

    const staticDirectory = await screen.findByLabelText("Static directory");
    await user.clear(staticDirectory);
    await user.type(staticDirectory, "/draft/static");

    await act(async () => {
      queryClient.setQueryData([GENERAL_SETTINGS_KEY], {
        error: false,
        data: {
          settings: {
            ...generalSettings(),
            static_dir: "/srv/igloo/static",
            transcode_dir: "/srv/igloo/transcode",
          },
        },
      });
    });

    expect(screen.getByLabelText("Static directory")).toHaveValue(
      "/draft/static",
    );

    await user.click(screen.getByRole("button", { name: "Reset" }));

    expect(screen.getByLabelText("Static directory")).toHaveValue(
      "/srv/igloo/static",
    );
    expect(screen.getByRole("button", { name: "Reset" })).toBeDisabled();
    expect(screen.getByText("No unsaved changes")).toBeInTheDocument();
  });

  it("drops a field's validation error once that field is edited, even back to its saved value", async () => {
    const user = userEvent.setup();
    await renderSettingsRoute("/settings");

    const jellyfin = await screen.findByRole("textbox", {
      name: "Jellyfin base URL",
    });
    const immich = screen.getByRole("textbox", { name: "Immich base URL" });
    const message = "Jellyfin base URL must start with http:// or https://.";

    await user.type(jellyfin, "ftp://not-valid.local");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));
    expect(jellyfin).toHaveAttribute("aria-invalid", "true");
    expect(screen.getAllByText(message).length).toBeGreaterThan(0);

    // An edit elsewhere leaves the flagged field's error alone.
    await user.type(immich, "h");
    expect(jellyfin).toHaveAttribute("aria-invalid", "true");
    await user.clear(immich);

    // Restoring the saved (empty) value leaves nothing to save or reset, so
    // the error has to go with the edit.
    await user.clear(jellyfin);
    expect(jellyfin).not.toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByText(message)).toBeNull();
    expect(screen.getByText("No unsaved changes")).toBeInTheDocument();
  });

  it("updates a clean libraries settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings/libraries");

    expect(await screen.findByLabelText("Movies library path")).toHaveValue(
      "/media/movies",
    );

    await act(async () => {
      queryClient.setQueryData([SETTINGS_KEY], {
        error: false,
        data: {
          ...librarySettings(),
          movies_dir: "/srv/movies",
          music_dir: "/srv/music",
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Movies library path")).toHaveValue(
        "/srv/movies",
      );
    });
  });

  it("preserves dirty library path edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings/libraries");

    const moviesPath = await screen.findByLabelText("Movies library path");
    await user.clear(moviesPath);
    await user.type(moviesPath, "/draft/movies");

    await act(async () => {
      queryClient.setQueryData([SETTINGS_KEY], {
        error: false,
        data: {
          ...librarySettings(),
          movies_dir: "/srv/movies",
          music_dir: "/srv/music",
        },
      });
    });

    expect(screen.getByLabelText("Movies library path")).toHaveValue(
      "/draft/movies",
    );

    await user.click(
      screen.getByRole("button", { name: "Reset library paths" }),
    );

    expect(screen.getByLabelText("Movies library path")).toHaveValue(
      "/srv/movies",
    );
  });

  it("updates a clean playback settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings/playback");

    expect(
      await screen.findByLabelText("Server upload bandwidth (Mbps)"),
    ).toHaveValue(20);

    await act(async () => {
      queryClient.setQueryData([PLAYBACK_SETTINGS_KEY], {
        error: false,
        data: {
          settings: playbackSettings({ server_upload_mbps: 25 }),
        },
      });
    });

    await waitFor(() => {
      expect(
        screen.getByLabelText("Server upload bandwidth (Mbps)"),
      ).toHaveValue(25);
    });
  });

  it("preserves dirty playback settings edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings/playback");

    const serverUpload = await screen.findByLabelText(
      "Server upload bandwidth (Mbps)",
    );
    await user.clear(serverUpload);
    await user.type(serverUpload, "12.5");

    await act(async () => {
      queryClient.setQueryData([PLAYBACK_SETTINGS_KEY], {
        error: false,
        data: {
          settings: playbackSettings({ server_upload_mbps: 25 }),
        },
      });
    });

    expect(
      screen.getByLabelText("Server upload bandwidth (Mbps)"),
    ).toHaveValue(12.5);

    await user.click(screen.getByRole("button", { name: "Reset" }));

    expect(
      screen.getByLabelText("Server upload bandwidth (Mbps)"),
    ).toHaveValue(25);
  });
});
