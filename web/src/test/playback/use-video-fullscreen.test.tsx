import { useRef } from "react";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { toast } from "sonner";
import { useVideoFullscreen } from "@/hooks/useVideoFullscreen";

vi.mock("sonner", () => ({
  toast: {
    info: vi.fn(),
  },
}));

const fullscreenElementDescriptor = Object.getOwnPropertyDescriptor(
  document,
  "fullscreenElement",
);
const webkitFullscreenElementDescriptor = Object.getOwnPropertyDescriptor(
  document,
  "webkitFullscreenElement",
);

let fullscreenElement: Element | null = null;

function restoreDocumentProperty(
  key: "fullscreenElement" | "webkitFullscreenElement",
  descriptor: PropertyDescriptor | undefined,
) {
  if (descriptor) {
    Object.defineProperty(document, key, descriptor);
    return;
  }

  delete (document as unknown as Record<string, unknown>)[key];
}

function VideoFullscreenHarness() {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode,
    toggleFullscreen,
  } = useVideoFullscreen({ containerRef, videoRef });

  return (
    <div>
      <div ref={containerRef} data-testid="container">
        <video ref={videoRef} data-testid="video" />
      </div>
      <div data-testid="is-fullscreen">{String(isFullscreen)}</div>
      <div data-testid="is-immersive">{String(isImmersiveViewport)}</div>
      <div data-testid="chrome-mode">{String(chromeFullscreenMode)}</div>
      <button type="button" onClick={() => void toggleFullscreen()}>
        Toggle fullscreen
      </button>
      <button
        type="button"
        onClick={() => void toggleFullscreen({ videoFallback: false })}
      >
        Toggle fullscreen without the video
      </button>
    </div>
  );
}

// The video mounts after the page's first render (a watch room once it has
// loaded, the movie after its pre-roll) and can be replaced by a new element.
function LateVideoHarness({ videoKey }: { videoKey: string | null }) {
  const containerRef = useRef<HTMLDivElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const { isFullscreen, toggleFullscreen, exitFullscreenIfActive } =
    useVideoFullscreen({ containerRef, videoRef });

  return (
    <div ref={containerRef}>
      {videoKey ? <video key={videoKey} ref={videoRef} data-testid="video" /> : null}
      <div data-testid="is-fullscreen">{String(isFullscreen)}</div>
      <button type="button" onClick={() => void toggleFullscreen()}>
        Toggle fullscreen
      </button>
      <button
        type="button"
        onClick={(event) => {
          event.currentTarget.dataset.handled = String(exitFullscreenIfActive());
        }}
      >
        Escape
      </button>
    </div>
  );
}

function setDisplayingFullscreen(video: HTMLElement, displaying: boolean) {
  Object.defineProperty(video, "webkitDisplayingFullscreen", {
    configurable: true,
    get: () => displaying,
  });
}

beforeEach(() => {
  fullscreenElement = null;

  Object.defineProperty(document, "fullscreenElement", {
    configurable: true,
    get: () => fullscreenElement,
  });
  Object.defineProperty(document, "webkitFullscreenElement", {
    configurable: true,
    get: () => null,
  });
});

afterEach(() => {
  fullscreenElement = null;
  document.body.style.overflow = "";
});

afterAll(() => {
  restoreDocumentProperty("fullscreenElement", fullscreenElementDescriptor);
  restoreDocumentProperty(
    "webkitFullscreenElement",
    webkitFullscreenElementDescriptor,
  );
});

describe("useVideoFullscreen", () => {
  it("derives fullscreen flags from document fullscreen events", () => {
    render(<VideoFullscreenHarness />);

    const container = screen.getByTestId("container");

    act(() => {
      fullscreenElement = container;
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("true");

    act(() => {
      fullscreenElement = null;
      document.dispatchEvent(new Event("fullscreenchange"));
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("false");
  });

  it("derives fullscreen flags from WebKit video fullscreen events", () => {
    render(<VideoFullscreenHarness />);

    const video = screen.getByTestId("video");

    act(() => {
      video.dispatchEvent(new Event("webkitbeginfullscreen"));
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("true");

    act(() => {
      video.dispatchEvent(new Event("webkitendfullscreen"));
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("false");
  });

  it("hears WebKit video fullscreen events from a video mounted or replaced later", () => {
    const { rerender } = render(<LateVideoHarness videoKey={null} />);

    rerender(<LateVideoHarness videoKey="first" />);
    const first = screen.getByTestId("video");
    act(() => {
      first.dispatchEvent(new Event("webkitbeginfullscreen"));
    });
    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
    act(() => {
      first.dispatchEvent(new Event("webkitendfullscreen"));
    });
    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");

    rerender(<LateVideoHarness videoKey="second" />);
    const second = screen.getByTestId("video");
    expect(second).not.toBe(first);
    act(() => {
      second.dispatchEvent(new Event("webkitbeginfullscreen"));
    });
    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
  });

  it("exits WebKit video fullscreen through the video while it is showing", async () => {
    render(<LateVideoHarness videoKey="ours" />);
    const video = screen.getByTestId("video");
    const webkitExitFullscreen = vi.fn();
    Object.defineProperty(video, "webkitExitFullscreen", {
      configurable: true,
      value: webkitExitFullscreen,
    });
    setDisplayingFullscreen(video, true);
    act(() => {
      video.dispatchEvent(new Event("webkitbeginfullscreen"));
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Toggle fullscreen" }));
    });

    expect(webkitExitFullscreen).toHaveBeenCalledTimes(1);
    // The video's own end event is what clears the mode.
    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
  });

  // Torn down inside WebKit video-only fullscreen, the video fires its end
  // event detached, where the document listener never hears it. The mode
  // clears once the video has left the document, with or without a new one.
  describe("after the video is torn down inside WebKit video fullscreen", () => {
    function enterThenTearDown(nextKey: string | null) {
      const { rerender } = render(<LateVideoHarness videoKey="first" />);
      const video = screen.getByTestId("video");
      setDisplayingFullscreen(video, true);
      act(() => {
        video.dispatchEvent(new Event("webkitbeginfullscreen"));
      });
      expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("true");
      setDisplayingFullscreen(video, false);
      rerender(<LateVideoHarness videoKey={nextKey} />);
      act(() => {
        video.dispatchEvent(new Event("webkitendfullscreen"));
      });
    }

    it("clears the mode once the video leaves the document", async () => {
      enterThenTearDown(null);

      await waitFor(() => {
        expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
      });
    });

    it("clears the mode when a new video replaces it", async () => {
      enterThenTearDown("second");

      await waitFor(() => {
        expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
      });
      // Escape then has nothing to exit.
      const escape = screen.getByRole("button", { name: "Escape" });
      act(() => {
        fireEvent.click(escape);
      });
      expect(escape.dataset.handled).toBe("false");
    });
  });

  it("ignores WebKit fullscreen events from another video on the page", () => {
    render(<LateVideoHarness videoKey="ours" />);
    const other = document.createElement("video");
    document.body.append(other);

    act(() => {
      other.dispatchEvent(new Event("webkitbeginfullscreen"));
    });
    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
    other.remove();
  });

  it("uses immersive viewport fallback when browser fullscreen is unavailable", async () => {
    render(<VideoFullscreenHarness />);

    const container = screen.getByTestId("container");
    Object.defineProperty(container, "requestFullscreen", {
      configurable: true,
      value: undefined,
    });
    Object.defineProperty(container, "webkitRequestFullscreen", {
      configurable: true,
      value: undefined,
    });

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Toggle fullscreen" }),
      );
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("true");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("true");
    expect(document.body.style.overflow).toBe("hidden");
    // Through showInfo, which passes no options for a toast without a
    // description.
    expect(toast.info).toHaveBeenCalledExactlyOnceWith(
      "Full screen isn't available in this browser. Using expanded view instead.",
      undefined,
    );

    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Toggle fullscreen" }),
      );
    });

    expect(screen.getByTestId("is-fullscreen")).toHaveTextContent("false");
    expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    expect(screen.getByTestId("chrome-mode")).toHaveTextContent("false");
    expect(document.body.style.overflow).toBe("");
  });

  describe("on a browser with only WebKit video fullscreen (iPhone Safari)", () => {
    function renderWithoutElementFullscreen() {
      render(<VideoFullscreenHarness />);
      const container = screen.getByTestId("container");
      Object.defineProperty(container, "requestFullscreen", {
        configurable: true,
        value: undefined,
      });
      Object.defineProperty(container, "webkitRequestFullscreen", {
        configurable: true,
        value: undefined,
      });
      const webkitEnterFullscreen = vi.fn();
      Object.defineProperty(screen.getByTestId("video"), "webkitEnterFullscreen", {
        configurable: true,
        value: webkitEnterFullscreen,
      });
      return { webkitEnterFullscreen };
    }

    it("takes the video element fullscreen", async () => {
      const { webkitEnterFullscreen } = renderWithoutElementFullscreen();

      await act(async () => {
        fireEvent.click(
          screen.getByRole("button", { name: "Toggle fullscreen" }),
        );
      });

      expect(webkitEnterFullscreen).toHaveBeenCalledTimes(1);
      expect(screen.getByTestId("is-immersive")).toHaveTextContent("false");
    });

    it("expands the container instead when the video is not what plays", async () => {
      // The trailer pre-roll: the element is the movie warming up underneath.
      const { webkitEnterFullscreen } = renderWithoutElementFullscreen();

      await act(async () => {
        fireEvent.click(
          screen.getByRole("button", {
            name: "Toggle fullscreen without the video",
          }),
        );
      });

      expect(webkitEnterFullscreen).not.toHaveBeenCalled();
      expect(screen.getByTestId("is-immersive")).toHaveTextContent("true");
    });
  });
});
