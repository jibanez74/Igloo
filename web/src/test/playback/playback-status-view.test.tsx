import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import PlaybackStatusView from "@/components/playback/PlaybackStatus";
import type { PlaybackStatus } from "@/types";

function renderStatus(
  status: Exclude<PlaybackStatus, { kind: "ready" }>,
  mediaNoun = "episode",
) {
  const onBack = vi.fn();
  const onRetry = vi.fn();
  const backButtonRef = createRef<HTMLButtonElement>();
  render(
    <PlaybackStatusView
      status={status}
      mediaNoun={mediaNoun}
      onBack={onBack}
      onRetry={onRetry}
      backButtonRef={backButtonRef}
      containerRef={createRef<HTMLDivElement>()}
    />,
  );
  return { onBack, onRetry, backButtonRef };
}

describe("PlaybackStatusView", () => {
  it("names the missing media and offers only Back", async () => {
    const user = userEvent.setup();
    const { onBack, backButtonRef } = renderStatus({ kind: "notFound" });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Episode not found");
    expect(alert).toHaveTextContent(
      "The episode could not be found or you don't have access to it.",
    );
    const back = screen.getByRole("button", { name: "Back to previous page" });
    expect(screen.getAllByRole("button")).toEqual([back]);
    expect(backButtonRef.current).toBe(back);

    await user.click(back);
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("names the quality that is not available", () => {
    renderStatus({ kind: "modeUnavailable", modeLabel: "4K · 40 Mbps" }, "movie");

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Quality not available");
    expect(alert).toHaveTextContent(
      "4K · 40 Mbps is not available for this movie.",
    );
    expect(screen.getAllByRole("button")).toHaveLength(1);
  });

  it("offers Try Again before Back after a failure", async () => {
    const user = userEvent.setup();
    const { onBack, onRetry } = renderStatus({
      kind: "error",
      message: "The stream stopped responding.",
    });

    expect(screen.getByRole("alert")).toHaveTextContent("Playback failed");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "The stream stopped responding.",
    );
    const buttons = screen.getAllByRole("button");
    expect(buttons.map(button => button.getAttribute("aria-label"))).toEqual([
      "Try again",
      "Back to previous page",
    ]);

    await user.click(buttons[0]);
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onBack).not.toHaveBeenCalled();
    await user.click(buttons[1]);
    expect(onBack).toHaveBeenCalledOnce();
  });

  it("shows the loading message without actions or an alert", () => {
    renderStatus({ kind: "loading", message: "Preparing playback..." });

    expect(screen.getByText("Preparing playback...")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
