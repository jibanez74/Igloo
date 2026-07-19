import { useRef } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
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
    </div>
  );
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
    expect(toast.info).toHaveBeenCalledWith(
      "Full screen isn't available in this browser. Using expanded view instead.",
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
});
