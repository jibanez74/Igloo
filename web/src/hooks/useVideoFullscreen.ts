import { useEffect, useReducer, useRef } from "react";
import type { RefObject } from "react";
import { toast } from "sonner";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  requestElementFullscreen,
  tryWebKitVideoEnterFullscreen,
  tryWebKitVideoExitFullscreen,
} from "@/lib/fullscreen";

type VideoFullscreenOptions = {
  containerRef: RefObject<HTMLDivElement | null>;
  videoRef: RefObject<HTMLVideoElement | null>;
};

type FullscreenMode = "none" | "document" | "webkitVideo" | "immersiveViewport";

type ToggleFullscreenOptions = {
  /**
   * Whether a browser without element fullscreen may take the video element
   * fullscreen on its own. The trailer pre-roll turns this off: the element
   * there is the movie warming up underneath, not what the viewer is watching.
   */
  videoFallback?: boolean;
};

function fullscreenModeReducer(
  mode: FullscreenMode,
  nextMode: FullscreenMode,
): FullscreenMode {
  return mode === nextMode ? mode : nextMode;
}

export function useVideoFullscreen({
  containerRef,
  videoRef,
}: VideoFullscreenOptions) {
  const [fullscreenMode, setFullscreenMode] = useReducer(
    fullscreenModeReducer,
    "none",
  );
  const fullscreenModeRef = useRef<FullscreenMode>("none");

  const isFullscreen =
    fullscreenMode === "document" || fullscreenMode === "webkitVideo";
  const isImmersiveViewport = fullscreenMode === "immersiveViewport";
  const chromeFullscreenMode = fullscreenMode !== "none";

  useEffect(() => {
    const onFullscreenChange = () => {
      const entering = !!getFullscreenElement();
      if (entering) {
        fullscreenModeRef.current = "document";
        setFullscreenMode("document");
      } else {
        if (fullscreenModeRef.current === "document") {
          fullscreenModeRef.current = "none";
          setFullscreenMode("none");
        }
      }
    };
    document.addEventListener("fullscreenchange", onFullscreenChange);
    document.addEventListener("webkitfullscreenchange", onFullscreenChange);
    return () => {
      document.removeEventListener("fullscreenchange", onFullscreenChange);
      document.removeEventListener(
        "webkitfullscreenchange",
        onFullscreenChange,
      );
    };
  }, []);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onWebKitBegin = () => {
      fullscreenModeRef.current = "webkitVideo";
      setFullscreenMode("webkitVideo");
    };
    const onWebKitEnd = () => {
      fullscreenModeRef.current = "none";
      setFullscreenMode("none");
    };

    video.addEventListener("webkitbeginfullscreen", onWebKitBegin);
    video.addEventListener("webkitendfullscreen", onWebKitEnd);
    return () => {
      video.removeEventListener("webkitbeginfullscreen", onWebKitBegin);
      video.removeEventListener("webkitendfullscreen", onWebKitEnd);
    };
  }, [videoRef]);

  useEffect(() => {
    if (!isImmersiveViewport) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previousOverflow;
    };
  }, [isImmersiveViewport]);

  // The container is what goes fullscreen; the video element only matters
  // for the WebKit video-only paths, and it is absent while the trailer
  // pre-roll plays, which must still be able to go fullscreen.
  const toggleFullscreen = async ({
    videoFallback = true,
  }: ToggleFullscreenOptions = {}) => {
    const container = containerRef.current;
    const video = videoRef.current;
    if (!container) return;

    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return;
    }
    if (fullscreenModeRef.current === "webkitVideo") {
      if (video) tryWebKitVideoExitFullscreen(video);
      return;
    }
    if (fullscreenModeRef.current === "immersiveViewport") {
      fullscreenModeRef.current = "none";
      setFullscreenMode("none");
      return;
    }

    const enterFallback = () => {
      if (videoFallback && video && tryWebKitVideoEnterFullscreen(video)) {
        return;
      }
      fullscreenModeRef.current = "immersiveViewport";
      setFullscreenMode("immersiveViewport");
      toast.info(
        "Full screen isn't available in this browser. Using expanded view instead.",
      );
    };

    if (
      !canRequestElementFullscreen(container) ||
      !isDocumentFullscreenEntryLikely()
    ) {
      enterFallback();
      return;
    }

    try {
      await requestElementFullscreen(container);
    } catch {
      enterFallback();
    }
  };

  const exitFullscreenIfActive = (): boolean => {
    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return true;
    }
    const video = videoRef.current;
    if (
      fullscreenModeRef.current === "webkitVideo" &&
      video &&
      tryWebKitVideoExitFullscreen(video)
    ) {
      return true;
    }
    if (fullscreenModeRef.current === "immersiveViewport") {
      fullscreenModeRef.current = "none";
      setFullscreenMode("none");
      return true;
    }
    return false;
  };

  return {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode,
    toggleFullscreen,
    exitFullscreenIfActive,
  };
}
