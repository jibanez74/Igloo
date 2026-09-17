import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useIdleControls } from "@/hooks/useIdleControls";

type IdleControlsHarnessProps = {
  active: boolean;
  idleMs: number;
};

function IdleControlsHarness({ active, idleMs }: IdleControlsHarnessProps) {
  const { visible, showAndReset } = useIdleControls({ active, idleMs });

  return (
    <div>
      <div data-testid="controls-visible">{String(visible)}</div>
      <button type="button" onClick={showAndReset}>
        Activity
      </button>
    </div>
  );
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe("useIdleControls", () => {
  it("hides controls after the active idle delay", async () => {
    vi.useFakeTimers();

    render(<IdleControlsHarness active idleMs={1000} />);

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    await act(async () => {
      vi.advanceTimersByTime(999);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("false");
  });

  it("resets the hide delay when activity is reported", async () => {
    vi.useFakeTimers();

    render(<IdleControlsHarness active idleMs={1000} />);

    await act(async () => {
      vi.advanceTimersByTime(900);
    });
    fireEvent.click(screen.getByRole("button", { name: "Activity" }));

    await act(async () => {
      vi.advanceTimersByTime(999);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("false");
  });

  it("keeps controls visible while inactive and resets on re-entry", async () => {
    vi.useFakeTimers();

    const { rerender } = render(
      <IdleControlsHarness active idleMs={1000} />,
    );

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("false");

    rerender(<IdleControlsHarness active={false} idleMs={1000} />);

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    rerender(<IdleControlsHarness active idleMs={1000} />);

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("true");

    await act(async () => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByTestId("controls-visible")).toHaveTextContent("false");
  });
});
