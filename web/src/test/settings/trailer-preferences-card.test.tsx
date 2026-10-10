import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import TrailerPreferencesCard from "@/components/settings/TrailerPreferencesCard";
import type { TrailerPreferencesData } from "@/types";
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

  it("rolls the control back and toasts when the save fails", async () => {
    const user = userEvent.setup();
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
      await new Promise((resolve) => setTimeout(resolve, 300));
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
});
