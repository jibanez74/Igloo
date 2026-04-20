import type { PropsWithChildren } from "react";
import { act, render, screen } from "@testing-library/react";
import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AppBoot from "@/AppBoot";
import { SPLASH_REMOVE_DELAY_MS } from "@/lib/app-boot";
import RouterPending from "@/components/RouterPending";

vi.mock("@/App", () => ({
  default: () => <div data-testid="app-shell" />,
}));

vi.mock("@/context/AudioPlayerContext", () => ({
  AudioPlayerProvider: ({ children }: PropsWithChildren) => children,
}));

vi.mock("sonner", () => ({
  Toaster: () => null,
}));

describe("App boot loading", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    document.documentElement.setAttribute("data-app-ready", "false");
    document.body.innerHTML = "";
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    document.documentElement.removeAttribute("data-app-ready");
    document.body.innerHTML = "";
  });

  it("marks the document ready and removes the initial splash after the fade window", () => {
    document.body.innerHTML = `
      <div
        id="initial-splash"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        <div class="initial-splash__content">
          <p class="initial-splash__title">Igloo</p>
          <p class="initial-splash__message">Loading your media library...</p>
        </div>
      </div>
      <div id="test-root"></div>
    `;

    render(<AppBoot queryClient={new QueryClient()} />, {
      container: document.getElementById("test-root")!,
    });

    expect(document.documentElement).toHaveAttribute(
      "data-app-ready",
      "true",
    );
    expect(document.getElementById("initial-splash")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(SPLASH_REMOVE_DELAY_MS - 1);
    });

    expect(document.getElementById("initial-splash")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });

    expect(document.getElementById("initial-splash")).not.toBeInTheDocument();
  });

  it("renders a single accessible loading surface for router pending", () => {
    render(<RouterPending />);

    const statuses = screen.getAllByRole("status");

    expect(statuses).toHaveLength(1);
    expect(statuses[0]).toHaveAttribute("aria-live", "polite");
    expect(screen.getByText("Igloo")).toBeInTheDocument();
    expect(
      screen.getByText("Loading your media library..."),
    ).toBeInTheDocument();
  });

  it("defers router pending live-region semantics while the initial splash is present", () => {
    document.body.innerHTML = `
      <div
        id="initial-splash"
        role="status"
        aria-live="polite"
        aria-atomic="true"
      >
        <div class="initial-splash__content">
          <p class="initial-splash__title">Igloo</p>
          <p class="initial-splash__message">Loading your media library...</p>
        </div>
      </div>
      <div id="test-root"></div>
    `;

    render(<RouterPending />, {
      container: document.getElementById("test-root")!,
    });

    const statuses = screen.getAllByRole("status");

    expect(statuses).toHaveLength(1);
    expect(statuses[0]).toHaveAttribute("id", "initial-splash");
  });
});
