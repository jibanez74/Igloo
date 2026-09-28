import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerNowPlayingContext } from "@/context/AudioPlayerContext";
import type { PlaybackSettingsType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";

function adminUser() {
  return {
    id: 1,
    name: "Admin",
    email: "admin@example.com",
    is_admin: true,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function playbackSettings(
  overrides: Partial<PlaybackSettingsType> = {},
): PlaybackSettingsType {
  return {
    profiles: [
      { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
      { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
    ],
    server_upload_mbps: null,
    hardware_acceleration_device: "nvidia",
    effective_hardware_acceleration_device: "nvidia",
    hardware_fallback_reason: "",
    max_transcode_height: 2160,
    ...overrides,
  };
}

async function renderPlaybackSettings(settings: PlaybackSettingsType) {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const url = requestURL(input);
      if (url === "/api/auth/user") {
        return jsonResponse({ error: false, data: { user: adminUser() } });
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
    await renderPlaybackSettings(playbackSettings());

    const notice = await screen.findByText(
      "In use for new transcodes: NVIDIA NVENC.",
    );
    expect(notice).toHaveClass("text-muted-foreground");
    expect(notice).not.toHaveTextContent("capped");
  });

  it("names the CPU cap when transcodes run on the CPU", async () => {
    await renderPlaybackSettings(
      playbackSettings({
        hardware_acceleration_device: "cpu",
        effective_hardware_acceleration_device: "cpu",
        max_transcode_height: 1080,
      }),
    );

    const notice = await screen.findByText(/In use for new transcodes: CPU\./);
    expect(notice).toHaveTextContent(
      "Software transcodes are capped at 1080p.",
    );
  });

  it("flags a stored device the startup probe refused", async () => {
    await renderPlaybackSettings(
      playbackSettings({
        effective_hardware_acceleration_device: "cpu",
        hardware_fallback_reason: "h264_nvenc runtime probe failed",
        max_transcode_height: 1080,
      }),
    );

    const notice = await screen.findByText(/is not available on this server/);
    expect(notice).toHaveClass("text-destructive");
    expect(notice).toHaveTextContent(
      "NVIDIA NVENC is not available on this server (h264_nvenc runtime probe failed), so transcodes run on the CPU. Software transcodes are capped at 1080p.",
    );
  });
});
