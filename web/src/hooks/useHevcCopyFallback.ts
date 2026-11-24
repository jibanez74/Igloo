import { useEffect, useRef } from "react";

type HevcCopyFallbackOptions = {
  streamWindowKey: string;
  /** True while the stream's request declares `video_codecs=hevc`. */
  hevcCopyDeclared: boolean;
  onFallback: () => void;
};

/**
 * One-shot fallback from a copied HEVC stream the browser failed to decode
 * to a transcode, which always encodes H.264. The capability probe can say
 * yes for a decoder that then refuses the stream (a missing hardware decoder
 * behind a codec the platform lists, a profile the probe did not ask about).
 *
 * `handleDecodeFailure` returns true when the failure is this hook's: the
 * first one in a stream window starts the fallback, and any later one in the
 * same window (hls.js and the element can both report the same failure)
 * is swallowed while the switch is under way.
 */
export function useHevcCopyFallback({
  streamWindowKey,
  hevcCopyDeclared,
  onFallback,
}: HevcCopyFallbackOptions) {
  const attemptedRef = useRef(false);

  useEffect(() => {
    attemptedRef.current = false;
  }, [streamWindowKey]);

  const handleDecodeFailure = (): boolean => {
    if (!hevcCopyDeclared) return false;
    if (attemptedRef.current) return true;
    attemptedRef.current = true;
    onFallback();
    return true;
  };

  return { handleDecodeFailure };
}
