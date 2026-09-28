import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerNowPlayingContext } from "@/context/AudioPlayerContext";
import type { PlaybackSettingsType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser, playbackSettings } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

// A 4K-capable catalog, so a 1080p cap is below what the server offers.
const profilesWith2160p = [
  { id: "2160p_16mbps", label: "2160p · 16 Mbps", height: 2160, video_mbps: 16 },
  ...playbackSettings().profiles,
];

function nvidiaSettings(
  overrides: Partial<PlaybackSettingsType> = {},
): PlaybackSettingsType {
  return playbackSettings({
    profiles: profilesWith2160p,
    hardware_acceleration_device: "nvidia",
    effective_hardware_acceleration_device: "nvidia",
    max_transcode_height: 2160,
    ...overrides,
  });
}

async function renderPlaybackSettings(settings: PlaybackSettingsType) {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const url = requestURL(input);
      if (url === "/api/auth/user") {
        return jsonResponse(authUser({ is_admin: true }));
      }
      if (url === "/api/settings/playback") {
        return jsonResponse({ error: false, data: { settings } });
      }
      return jsonResponse(
        { error: true, message: `Unexpected request: ${url}` },
        500,
      );
    }),
  );

  await renderRoute("/settings/playback", {
    wrapper: children => (
      <AudioPlayerNowPlayingContext.Provider value={null}>
        {children}
      </AudioPlayerNowPlayingContext.Provider>
    ),
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

// The dropdown shows the stored device, but transcodes run on the effective
// one: the env seed does not survive an edit, and the startup probe can refuse
// the stored device. The notice reads from the saved settings, not the form.
describe("server playback form effective device notice", () => {
  it("confirms the stored device when the server runs it", async () => {
    await renderPlaybackSettings(nvidiaSettings());

    const notice = await screen.findByText(
      "In use for new transcodes: NVIDIA NVENC.",
    );
    expect(notice).toHaveClass("text-muted-foreground");
    expect(notice).not.toHaveTextContent("capped");
  });

  it("names the cap when it is below the catalog", async () => {
    await renderPlaybackSettings(
      playbackSettings({ profiles: profilesWith2160p }),
    );

    const notice = await screen.findByText(/In use for new transcodes: CPU\./);
    expect(notice).toHaveTextContent("Transcodes are capped at 1080p.");
  });

  it("flags a stored device the startup probe refused", async () => {
    await renderPlaybackSettings(
      nvidiaSettings({
        effective_hardware_acceleration_device: "cpu",
        hardware_fallback_reason: "h264_nvenc runtime probe failed",
        max_transcode_height: 1080,
      }),
    );

    const notice = await screen.findByText(/is not available on this server/);
    expect(notice).toHaveClass("text-destructive");
    expect(notice).toHaveTextContent(
      "NVIDIA NVENC is not available on this server (h264_nvenc runtime probe failed), so transcodes run on the CPU. Transcodes are capped at 1080p.",
    );
  });
});
