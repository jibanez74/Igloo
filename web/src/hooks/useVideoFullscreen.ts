import { useEffect, useRef, useState } from "react";
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

export function useVideoFullscreen({
  containerRef,
  videoRef,
}: VideoFullscreenOptions) {
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [isImmersiveViewport, setIsImmersiveViewport] = useState(false);
  const fullscreenSourceRef = useRef<"none" | "document" | "webkitVideo">(
    "none",
  );

  useEffect(() => {
    const onFullscreenChange = () => {
      const entering = !!getFullscreenElement();
      if (entering) {
        fullscreenSourceRef.current = "document";
        setIsFullscreen(true);
        setIsImmersiveViewport(false);
      } else {
        if (fullscreenSourceRef.current === "document") {
          fullscreenSourceRef.current = "none";
        }
        setIsFullscreen(false);
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
      fullscreenSourceRef.current = "webkitVideo";
      setIsFullscreen(true);
      setIsImmersiveViewport(false);
    };
    const onWebKitEnd = () => {
      fullscreenSourceRef.current = "none";
      setIsFullscreen(false);
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

  const toggleFullscreen = async () => {
    const container = containerRef.current;
    const video = videoRef.current;
    if (!container || !video) return;

    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return;
    }
    if (fullscreenSourceRef.current === "webkitVideo") {
      tryWebKitVideoExitFullscreen(video);
      return;
    }
    if (isImmersiveViewport) {
      setIsImmersiveViewport(false);
      return;
    }

    const enterFallback = () => {
      if (tryWebKitVideoEnterFullscreen(video)) {
        return;
      }
      setIsImmersiveViewport(true);
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
      fullscreenSourceRef.current === "webkitVideo" &&
      video &&
      tryWebKitVideoExitFullscreen(video)
    ) {
      return true;
    }
    if (isImmersiveViewport) {
      setIsImmersiveViewport(false);
      return true;
    }
    return false;
  };

  return {
    isFullscreen,
    isImmersiveViewport,
    chromeFullscreenMode: isFullscreen || isImmersiveViewport,
    toggleFullscreen,
    exitFullscreenIfActive,
  };
}
