import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import TrailerPreferencesCard from "@/components/settings/TrailerPreferencesCard";
import { LIVE_ANNOUNCE_DELAY_MS } from "@/lib/constants";
import type { TrailerPreferencesData } from "@/types";
import { mutationSettled, renderAcrossSessions } from "../helpers/session";
import { renderWithQueryClient } from "../helpers/render";

const getTrailerPreferencesMock = vi.fn();
const updateTrailerPreferencesMock = vi.fn();
const getTmdbStatusMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@/lib/api", () => ({
  getTrailerPreferences: (...args: unknown[]) =>
    getTrailerPreferencesMock(...args),
  updateTrailerPreferences: (...args: unknown[]) =>
    updateTrailerPreferencesMock(...args),
  getTmdbStatus: (...args: unknown[]) => getTmdbStatusMock(...args),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

const defaults: TrailerPreferencesData = {
  enabled: false,
  count: 2,
  source: "both",
};

function success(data: TrailerPreferencesData) {
  return { error: false as const, data };
}

function toggle() {
  return screen.getByRole("switch", { name: "Play trailers before movies" });
}

afterEach(() => {
  vi.useRealTimers();
});

beforeEach(() => {
  getTrailerPreferencesMock.mockResolvedValue(success(defaults));
  getTmdbStatusMock.mockResolvedValue({
    error: false,
    data: { available: true },
  });
  updateTrailerPreferencesMock.mockImplementation(
    async (prefs: TrailerPreferencesData) => success(prefs),
  );
});

describe("TrailerPreferencesCard", () => {
  it("renders the saved preferences with their account scope", async () => {
    renderWithQueryClient(<TrailerPreferencesCard />);

    expect(
      await screen.findByRole("heading", {
        level: 2,
        name: "Trailers before movies",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Saved to your account/)).toBeInTheDocument();
    expect(toggle()).toHaveAttribute("aria-checked", "false");
    expect(
      screen.getByRole("combobox", { name: "Number of trailers" }),
    ).toHaveTextContent("2 trailers");
    expect(
      screen.getByRole("combobox", { name: "Trailer source" }),
    ).toHaveTextContent("Both");
    expect(screen.queryByText(/TMDB is not configured/)).toBeNull();
  });

  it("saves the toggle to the account at once and announces it", async () => {
    const user = userEvent.setup();
    renderWithQueryClient(<TrailerPreferencesCard />);
    await screen.findByRole("switch");

    await user.click(toggle());

    await waitFor(() =>
      expect(updateTrailerPreferencesMock).toHaveBeenCalledWith({
        enabled: true,
        count: 2,
        source: "both",
      }),
    );
    expect(toggle()).toHaveAttribute("aria-checked", "true");
    expect(
      await screen.findByText(
        "Trailers before movies turned on. Saved to your account.",
      ),
    ).toBeInTheDocument();
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  // The switch used to be disabled for the length of every save, which
  // dropped keyboard focus in Chromium each time it was toggled.
  it("keeps focus on the switch while its save is in flight", async () => {
    let finish: (value: ReturnType<typeof success>) => void = () => {};
    updateTrailerPreferencesMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const user = userEvent.setup();
    renderWithQueryClient(<TrailerPreferencesCard />);
    await screen.findByRole("switch");

    toggle().focus();
    await user.keyboard(" ");

    await waitFor(() =>
      expect(updateTrailerPreferencesMock).toHaveBeenCalledTimes(1),
    );
    expect(toggle()).toBeEnabled();
    expect(toggle()).toHaveAttribute("aria-disabled", "true");
    expect(toggle()).toHaveFocus();

    // A second press while the first is saving does not start another save.
    await user.keyboard(" ");
    expect(updateTrailerPreferencesMock).toHaveBeenCalledTimes(1);

    finish(success({ ...defaults, enabled: true }));
    await waitFor(() => expect(toggle()).not.toHaveAttribute("aria-disabled"));
    expect(toggle()).toHaveFocus();
    expect(toggle()).toHaveAttribute("aria-checked", "true");
  });

  // A select hands focus back to its trigger when its list closes. Disabled
  // for the length of the save, the trigger could not take it (Firefox, on
  // any save slower than the list's closing).
  it("keeps both selects focusable while a save is in flight", async () => {
    let finish: (value: ReturnType<typeof success>) => void = () => {};
    updateTrailerPreferencesMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    );
    const user = userEvent.setup();
    renderWithQueryClient(<TrailerPreferencesCard />);
    await screen.findByRole("switch");
    const selects = () => [
      screen.getByRole("combobox", { name: "Number of trailers" }),
      screen.getByRole("combobox", { name: "Trailer source" }),
    ];

    await user.click(toggle());
    await waitFor(() =>
      expect(updateTrailerPreferencesMock).toHaveBeenCalledTimes(1),
    );
    for (const select of selects()) {
      expect(select).toBeEnabled();
      expect(select).toHaveAttribute("aria-disabled", "true");
    }

    // A pick made meanwhile starts no second save.
    selects()[0].focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("listbox");
    await user.click(screen.getByRole("option", { name: "3 trailers" }));
    await waitFor(() => expect(screen.queryByRole("listbox")).toBeNull());
    expect(updateTrailerPreferencesMock).toHaveBeenCalledTimes(1);

    finish(success({ ...defaults, enabled: true }));
    await waitFor(() =>
      expect(selects()[0]).not.toHaveAttribute("aria-disabled"),
    );
    expect(selects()[1]).not.toHaveAttribute("aria-disabled");
  });

  it("rolls the control back and toasts when the save fails", async () => {
    // Fake timers so the announcer's delay can be run out deterministically:
    // a "saved" message, had one been scheduled, would be spoken by then.
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    updateTrailerPreferencesMock.mockResolvedValue({
      error: true,
      message: "trailer count must be between 1 and 5",
    });
    renderWithQueryClient(<TrailerPreferencesCard />);
    await screen.findByRole("switch");

    await user.click(toggle());

    await waitFor(() =>
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "save trailer preferences",
        expect.objectContaining({
          error: true,
          message: "trailer count must be between 1 and 5",
        }),
      ),
    );
    expect(toggle()).toHaveAttribute("aria-checked", "false");
    await act(async () => {
      await vi.advanceTimersByTimeAsync(LIVE_ANNOUNCE_DELAY_MS);
    });
    expect(screen.queryByText(/Saved to your account\./)).toBeNull();
  });

  it("explains that theaters trailers fall back to the library without TMDB", async () => {
    getTmdbStatusMock.mockResolvedValue({
      error: false,
      data: { available: false },
    });
    renderWithQueryClient(<TrailerPreferencesCard />);

    const hint = await screen.findByText(
      /TMDB is not configured on this server/,
    );
    expect(hint).toHaveTextContent("Library trailers will be used instead.");
    expect(
      screen.getByRole("combobox", { name: "Trailer source" }),
    ).toHaveAccessibleDescription(hint.textContent ?? "");
  });

  it("words a server failure instead of showing its internal message", async () => {
    getTrailerPreferencesMock.mockResolvedValue({
      error: true,
      message: "sql: database is locked",
      status: 500,
    });
    renderWithQueryClient(<TrailerPreferencesCard />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Failed to load trailer preferences.");
    expect(alert).not.toHaveTextContent("database is locked");
  });

  it("keeps its place with a loading card, then an error card", async () => {
    getTrailerPreferencesMock.mockResolvedValue({
      error: true,
      message: "failed to load trailer preferences",
    });
    renderWithQueryClient(<TrailerPreferencesCard />);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Loading trailer preferences...",
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "failed to load trailer preferences",
    );
  });

  // Account A's save is still in flight when A signs out and B signs in. Its
  // answer belongs to A: it must not overwrite the preferences B just loaded,
  // roll anything back, or toast on B's page.
  it.each([
    ["success", { error: false as const, data: { enabled: true, count: 2, source: "both" as const } }],
    ["failure", { error: true as const, message: "trailer count must be between 1 and 5" }],
  ])("drops the previous account's %s", async (_label, answer) => {
    const user = userEvent.setup();
    let releaseSave: (value: unknown) => void = () => {};
    updateTrailerPreferencesMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          releaseSave = resolve;
        }),
    );
    const { queryClient, endSession, signIn } = await renderAcrossSessions(
      TrailerPreferencesCard,
    );
    await user.click(await screen.findByRole("switch"));
    await waitFor(() => expect(updateTrailerPreferencesMock).toHaveBeenCalled());

    await endSession();
    getTrailerPreferencesMock.mockResolvedValue(
      success({ enabled: false, count: 1, source: "library" }),
    );
    await signIn();
    await waitFor(() =>
      expect(
        screen.getByRole("combobox", { name: "Number of trailers" }),
      ).toHaveTextContent("1 trailer"),
    );

    const settled = mutationSettled(queryClient);
    releaseSave(answer);
    await settled;

    expect(toggle()).toHaveAttribute("aria-checked", "false");
    expect(
      screen.getByRole("combobox", { name: "Number of trailers" }),
    ).toHaveTextContent("1 trailer");
    expect(queryClient.getQueryData(["trailer-preferences"])).toEqual(
      success({ enabled: false, count: 1, source: "library" }),
    );
    expect(showActionFailedMock).not.toHaveBeenCalled();
    expect(getTrailerPreferencesMock).toHaveBeenCalledTimes(2);
  });
});
