import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useContentFadeTransition } from "@/hooks/useContentFadeTransition";

const defaultMatchMedia = window.matchMedia;
const originalStartViewTransition = (document as Document & {
  startViewTransition?: unknown;
}).startViewTransition;

type MatchMediaChangeListener = () => void;

function createReducedMotionController(initialMatches: boolean) {
  const listeners = new Set<MatchMediaChangeListener>();
  const addEventListener = vi.fn(
    (_type: string, listener: MatchMediaChangeListener) => {
      listeners.add(listener);
    },
  );
  const removeEventListener = vi.fn(
    (_type: string, listener: MatchMediaChangeListener) => {
      listeners.delete(listener);
    },
  );

  const mediaQueryList = {
    matches: initialMatches,
    media: "(prefers-reduced-motion: reduce)",
    onchange: null,
    addEventListener,
    removeEventListener,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  };

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => {
      if (query === "(prefers-reduced-motion: reduce)") {
        return mediaQueryList;
      }

      return {
        matches: false,
        media: query,
        onchange: null,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: vi.fn(),
      };
    }),
  });

  return {
    addEventListener,
    removeEventListener,
    setMatches(nextMatches: boolean) {
      mediaQueryList.matches = nextMatches;
      for (const listener of listeners) {
        listener();
      }
    },
  };
}

type HookHarnessProps = {
  enableViewTransition?: boolean;
  onTransition: () => void;
  transitionMs?: number;
};

function HookHarness({
  enableViewTransition = true,
  onTransition,
  transitionMs = 150,
}: HookHarnessProps) {
  const {
    isExiting,
    usesContentAnimation,
    usesViewTransition,
    runTransition,
  } = useContentFadeTransition(transitionMs, { enableViewTransition });

  return (
    <div>
      <div data-testid="is-exiting">{String(isExiting)}</div>
      <div data-testid="uses-content-animation">
        {String(usesContentAnimation)}
      </div>
      <div data-testid="uses-view-transition">{String(usesViewTransition)}</div>
      <button
        type="button"
        onClick={() => {
          runTransition({ onTransition });
        }}
      >
        Run transition
      </button>
    </div>
  );
}

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: defaultMatchMedia,
  });

  if (originalStartViewTransition === undefined) {
    delete (document as Document & { startViewTransition?: unknown })
      .startViewTransition;
  } else {
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: originalStartViewTransition,
    });
  }
});

describe("useContentFadeTransition", () => {
  it("delays the transition when reduced motion is off and view transitions are disabled", async () => {
    const reducedMotion = createReducedMotionController(false);
    const onTransition = vi.fn();

    vi.useFakeTimers();

    render(
      <HookHarness
        enableViewTransition={false}
        onTransition={onTransition}
        transitionMs={150}
      />,
    );

    expect(screen.getByTestId("uses-content-animation")).toHaveTextContent("true");
    expect(screen.getByTestId("uses-view-transition")).toHaveTextContent("false");
    expect(reducedMotion.addEventListener).toHaveBeenCalledWith(
      "change",
      expect.any(Function),
    );

    fireEvent.click(screen.getByRole("button", { name: "Run transition" }));

    expect(screen.getByTestId("is-exiting")).toHaveTextContent("true");
    expect(onTransition).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(149);
    });

    expect(onTransition).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(1);
    });

    expect(onTransition).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("is-exiting")).toHaveTextContent("false");
  });

  it("updates transition behavior when reduced motion changes after mount", async () => {
    const reducedMotion = createReducedMotionController(false);
    const onTransition = vi.fn();

    render(
      <HookHarness
        enableViewTransition={false}
        onTransition={onTransition}
        transitionMs={150}
      />,
    );

    act(() => {
      reducedMotion.setMatches(true);
    });

    fireEvent.click(screen.getByRole("button", { name: "Run transition" }));

    expect(onTransition).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("is-exiting")).toHaveTextContent("false");
  });

  it("switches away from view transitions when reduced motion changes at runtime", () => {
    const reducedMotion = createReducedMotionController(false);
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: vi.fn(),
    });
    const onTransition = vi.fn();

    render(<HookHarness onTransition={onTransition} transitionMs={150} />);

    expect(screen.getByTestId("uses-view-transition")).toHaveTextContent("true");
    expect(screen.getByTestId("uses-content-animation")).toHaveTextContent("false");

    act(() => {
      reducedMotion.setMatches(true);
    });

    expect(screen.getByTestId("uses-view-transition")).toHaveTextContent("false");
    expect(screen.getByTestId("uses-content-animation")).toHaveTextContent("true");
  });

  it("cleans up the reduced-motion listener on unmount", () => {
    const reducedMotion = createReducedMotionController(false);
    const onTransition = vi.fn();

    const { unmount } = render(
      <HookHarness onTransition={onTransition} transitionMs={150} />,
    );

    const changeListener = reducedMotion.addEventListener.mock.calls[0]?.[1];

    expect(changeListener).toEqual(expect.any(Function));

    unmount();

    expect(reducedMotion.removeEventListener).toHaveBeenCalledWith(
      "change",
      changeListener,
    );
  });
});
