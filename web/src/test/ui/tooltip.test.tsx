import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Tooltip, TooltipProvider } from "@/components/ui/tooltip";

function renderTooltip(props: Partial<React.ComponentProps<typeof Tooltip>> = {}) {
  return render(
    <Tooltip label="Switch to light theme" {...props}>
      <button type="button" aria-label="Switch to light theme">
        icon
      </button>
    </Tooltip>,
  );
}

describe("Tooltip", () => {
  it("shows nothing until the control is hovered or focused", () => {
    renderTooltip();

    expect(screen.queryByRole("tooltip")).toBeNull();
    expect(screen.getByRole("button", { name: "Switch to light theme" })).toBeVisible();
  });

  it("opens at once on keyboard focus and closes on blur", () => {
    renderTooltip();
    const button = screen.getByRole("button");

    act(() => button.focus());
    expect(screen.getByRole("tooltip")).toHaveTextContent("Switch to light theme");

    act(() => button.blur());
    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  it("shows nothing while disabled, without remounting the control", () => {
    const { rerender } = renderTooltip({ disabled: true });
    const button = screen.getByRole("button");

    act(() => button.focus());
    expect(screen.queryByRole("tooltip")).toBeNull();

    rerender(
      <Tooltip label="Switch to light theme">
        <button type="button" aria-label="Switch to light theme">
          icon
        </button>
      </Tooltip>,
    );
    expect(screen.getByRole("button")).toBe(button);
    act(() => button.blur());
    act(() => button.focus());
    expect(screen.getByRole("tooltip")).toHaveTextContent("Switch to light theme");
  });

  it("closes on Escape", () => {
    renderTooltip();
    const button = screen.getByRole("button");
    act(() => button.focus());
    expect(screen.getByRole("tooltip")).toBeInTheDocument();

    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(screen.queryByRole("tooltip")).toBeNull();
  });

  // The tooltip repeats the control's name; announcing it as a description
  // too would read the name twice.
  it("does not describe a control it only names", () => {
    renderTooltip({ open: true });

    const button = screen.getByRole("button", { name: "Switch to light theme" });
    expect(screen.getByRole("tooltip")).toBeInTheDocument();
    expect(button).not.toHaveAttribute("aria-describedby");
    expect(button).toHaveAccessibleDescription("");
  });

  it("describes the control when it adds to its name", () => {
    render(
      <Tooltip label="TMDB search is unavailable on this server." describes open>
        <button type="button">Request Movie</button>
      </Tooltip>,
    );

    const button = screen.getByRole("button", { name: "Request Movie" });
    expect(button).toHaveAccessibleDescription(
      "TMDB search is unavailable on this server.",
    );
  });

  it("keeps a description the control already had", () => {
    render(
      <>
        <p id="hint">Applies to this browser only.</p>
        <Tooltip label="Switch to light theme" open>
          <button type="button" aria-label="Switch to light theme" aria-describedby="hint">
            icon
          </button>
        </Tooltip>
      </>,
    );

    expect(screen.getByRole("button")).toHaveAccessibleDescription(
      "Applies to this browser only.",
    );
  });

  it("passes the control's own handlers and ref through", () => {
    let clicks = 0;
    let node: HTMLButtonElement | null = null;
    render(
      <Tooltip label="Notifications">
        <button
          type="button"
          aria-label="Notifications"
          ref={(element) => {
            node = element;
          }}
          onClick={() => {
            clicks += 1;
          }}
        >
          icon
        </button>
      </Tooltip>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Notifications" }));

    expect(clicks).toBe(1);
    expect(node).toBe(screen.getByRole("button"));
  });

  it("is drawn inverted, above every other layer, and holds still under reduced motion", () => {
    renderTooltip({ open: true });

    const bubble = document.querySelector("[data-slot='tooltip-content']");
    expect(bubble).toHaveClass(
      "z-popover",
      "bg-foreground",
      "text-background",
      "duration-micro",
      "motion-reduce:animate-none",
    );
  });

  it("works under the app's provider and without one", () => {
    render(
      <TooltipProvider>
        <Tooltip label="Notifications" open>
          <button type="button" aria-label="Notifications">
            icon
          </button>
        </Tooltip>
      </TooltipProvider>,
    );

    expect(screen.getByRole("tooltip")).toHaveTextContent("Notifications");
  });
});
