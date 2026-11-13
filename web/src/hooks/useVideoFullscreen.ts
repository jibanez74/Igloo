import { useEffect, useReducer, useRef } from "react";
import type { RefObject } from "react";
import { showInfo } from "@/lib/toast-helpers";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  isWebKitVideoFullscreen,
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

  // The WebKit video-only fullscreen events fire on the <video>, which mounts
  // after a room loads or a pre-roll ends, and is replaced on a source swap,
  // while videoRef stays the same object. Listening on the document in the
  // capture phase reaches whichever element is current (capture runs for
  // events that do not bubble), so no mount is ever missed.
  useEffect(() => {
    const isOurVideo = (event: Event) =>
      videoRef.current !== null && event.target === videoRef.current;

    const onWebKitBegin = (event: Event) => {
      if (!isOurVideo(event)) return;
      fullscreenModeRef.current = "webkitVideo";
      setFullscreenMode("webkitVideo");
    };
    const onWebKitEnd = (event: Event) => {
      if (!isOurVideo(event)) return;
      fullscreenModeRef.current = "none";
      setFullscreenMode("none");
    };

    document.addEventListener("webkitbeginfullscreen", onWebKitBegin, true);
    document.addEventListener("webkitendfullscreen", onWebKitEnd, true);
    return () => {
      document.removeEventListener("webkitbeginfullscreen", onWebKitBegin, true);
      document.removeEventListener("webkitendfullscreen", onWebKitEnd, true);
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

  // A video torn down inside WebKit video-only fullscreen (a playback error
  // replacing the player) fires its end event on a detached node, which
  // never reaches the document listener. Watching for that video to leave
  // the document clears the mode as it goes, so the controls never offer to
  // exit a fullscreen that has already ended.
  useEffect(() => {
    if (fullscreenMode !== "webkitVideo") return;
    const video = videoRef.current;
    if (!video) return;
    const observer = new MutationObserver(() => {
      if (video.isConnected || fullscreenModeRef.current !== "webkitVideo") {
        return;
      }
      fullscreenModeRef.current = "none";
      setFullscreenMode("none");
    });
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [fullscreenMode, videoRef]);

  // Should the mode still read "webkitVideo" with no video in fullscreen,
  // pressing the fullscreen toggle, or Escape, only clears it. Returns
  // whether it did.
  const dropStaleWebKitVideoMode = (): boolean => {
    if (fullscreenModeRef.current !== "webkitVideo") return false;
    const video = videoRef.current;
    if (video && isWebKitVideoFullscreen(video)) return false;
    fullscreenModeRef.current = "none";
    setFullscreenMode("none");
    return true;
  };

  // The container is what goes fullscreen; the video element only matters
  // for the WebKit video-only paths, and it is absent while the trailer
  // pre-roll plays, which must still be able to go fullscreen.
  const toggleFullscreen = async ({
    videoFallback = true,
  }: ToggleFullscreenOptions = {}) => {
    const container = containerRef.current;
    const video = videoRef.current;
    if (!container) return;

    if (dropStaleWebKitVideoMode()) return;
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
      showInfo(
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
    if (dropStaleWebKitVideoMode()) return true;
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
