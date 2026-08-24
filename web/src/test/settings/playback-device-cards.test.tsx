import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DevicePlaybackCards from "@/components/settings/DevicePlaybackCards";
import {
  resetDevicePlaybackPreferencesCache,
  setDevicePlaybackPreferences,
  storageKeyForUser,
} from "@/lib/playback-preferences";
import type { PlaybackSettingsType } from "@/types";

const USER_ID = 1;

const SETTINGS: PlaybackSettingsType = {
  profiles: [
    { id: "1080p_8mbps", label: "1080p · 8 Mbps", height: 1080, video_mbps: 8 },
    { id: "720p_3mbps", label: "720p · 3 Mbps", height: 720, video_mbps: 3 },
  ],
  server_upload_mbps: null,
  hardware_acceleration_device: "cpu",
};

// Matches only the standing card notice: the live announcement shares the
// "not saving settings" wording, so key off the sentence unique to the notice.
const FAILURE_NOTICE = /lost when the page reloads/i;

function downloadInput() {
  return screen.getByRole("spinbutton", {
    name: "Download speed (Mbps)",
  });
}

// LiveAnnouncer waits out a short delay before it writes into a live region,
// so an absent announcement can only be asserted after that window passes.
async function expectNoAnnouncement() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 300));
  });
  expect(screen.queryByText(/Download speed set to/)).toBeNull();
}

function otherTabWritesDownloadMbps(downloadMbps: number) {
  const key = storageKeyForUser(USER_ID);
  const newValue = JSON.stringify({ downloadMbps });
  localStorage.setItem(key, newValue);
  window.dispatchEvent(new StorageEvent("storage", { key, newValue }));
}

// The download-speed field is a native input, so it drives the same
// write-then-announce path the Radix selects use without needing a real
// pointer environment. The selects are covered by web/e2e/playback-settings.spec.ts.
async function setDownloadSpeed(mbps: string) {
  const user = userEvent.setup();
  const input = downloadInput();
  await user.clear(input);
  await user.type(input, mbps);
  await user.tab();
}

describe("device playback cards", () => {
  beforeEach(() => {
    localStorage.clear();
    resetDevicePlaybackPreferencesCache();
    vi.restoreAllMocks();
  });

  function renderCards() {
    render(
      <DevicePlaybackCards
        settings={SETTINGS}
        userId={USER_ID}
        isAdmin={false}
      />,
    );
  }

  it("confirms a durable save when localStorage accepts the write", async () => {
    renderCards();
    await setDownloadSpeed("25");

    // LiveAnnouncer double-buffers into two status regions, so assert on the
    // announced text rather than on a single region.
    await screen.findByText("Download speed set to 25 Mbps. Saved on this device.");
    expect(screen.queryAllByText(FAILURE_NOTICE)).toHaveLength(0);
  });

  // Blur is the commit point, so it runs even when the user only passed
  // through the field. Confirming a save there would announce a write that
  // never happened.
  it("confirms nothing when the field is blurred without an edit", async () => {
    setDevicePlaybackPreferences(USER_ID, { downloadMbps: 25 });
    const user = userEvent.setup();
    renderCards();

    expect(downloadInput()).toHaveDisplayValue("25");
    await user.click(downloadInput());
    await user.tab();

    expect(downloadInput()).not.toHaveFocus();
    await expectNoAnnouncement();
  });

  // The field holds its own text while typing, so a commit has to hand it back
  // to the stored preference -- otherwise it keeps rendering what was typed and
  // ignores the value another tab just saved.
  it("follows another tab's value after committing its own", async () => {
    renderCards();
    await setDownloadSpeed("40");
    await screen.findByText(
      "Download speed set to 40 Mbps. Saved on this device.",
    );

    act(() => {
      otherTabWritesDownloadMbps(15);
    });

    await waitFor(() => {
      expect(downloadInput()).toHaveDisplayValue("15");
    });
  });

  it("reports a session-only save and warns while storage refuses writes", async () => {
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota exceeded");
    });

    renderCards();
    await setDownloadSpeed("25");

    await screen.findByText(
      "Download speed set to 25 Mbps. Applied for this session only; this browser is not saving settings.",
    );
    expect(
      screen.queryByText(/Download speed set to 25 Mbps\. Saved on this device\./),
    ).toBeNull();
    // The announcement is one-shot, so both device cards carry the notice.
    expect(screen.getAllByText(FAILURE_NOTICE)).toHaveLength(2);
  });

  it("drops the warning once storage accepts writes again", async () => {
    const setItem = vi
      .spyOn(Storage.prototype, "setItem")
      .mockImplementation(() => {
        throw new Error("quota exceeded");
      });

    renderCards();
    await setDownloadSpeed("25");
    await waitFor(() => {
      expect(screen.getAllByText(FAILURE_NOTICE)).toHaveLength(2);
    });

    setItem.mockRestore();
    await setDownloadSpeed("40");

    await waitFor(() => {
      expect(screen.queryAllByText(FAILURE_NOTICE)).toHaveLength(0);
    });
    await screen.findByText("Download speed set to 40 Mbps. Saved on this device.");
  });
});
