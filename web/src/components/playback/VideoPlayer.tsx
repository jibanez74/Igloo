import { useEffect, useEffectEvent, useRef, useState } from "react";
import type { RefObject } from "react";
import type Hls from "hls.js";
import type { ErrorData } from "hls.js";
import type { Events } from "hls.js";
import { Spinner } from "@/components/ui/spinner";
import {
  HLS_ACTUAL_START_HEADER,
  HLS_CAPACITY_RETRY_FALLBACK_SEC,
  HLS_EFFECTIVE_PROFILE_HEADER,
  HLS_JS_BACK_BUFFER_LENGTH_SEC,
  HLS_JS_FRAG_LOAD_TIMEOUT_MS,
  HLS_JS_LOAD_TIMEOUT_MS,
  HLS_NETWORK_RECOVERY_DELAYS_MS,
  HLS_SEGMENT_NOT_READY_MAX_RETRIES,
  HLS_SEGMENT_STATUS_HEADER,
  HLS_SEGMENT_STATUS_PAST_END,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOVIE_BUFFERING_SPINNER_DELAY_MS,
} from "@/lib/constants";
import { prefersNativeHLS } from "@/lib/playback";
import { releaseResponseBody } from "@/lib/video-playback";
import { cn } from "@/lib/utils";

type SubtitleTrackInfo = {
  url: string;
  label: string;
  srclang: string;
};

type VideoPlayerProps = {
  videoRef: RefObject<HTMLVideoElement | null>;
  src: string;
  /** True when `src` is an HLS playlist rather than a direct-play file. */
  isHlsSource: boolean;
  title: string;
  isFullscreen?: boolean;
  onError: (message: string) => void;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onTimeUpdate?: (time: number) => void;
  onDurationChange?: (duration: number) => void;
  onNativeError?: (code: number | null | undefined) => void;
  subtitleTrack?: SubtitleTrackInfo | null;
  startSec?: number;
  /** Requested absolute start that identifies the current HLS session. */
  requestedStartSec?: number;
  onStartApplied?: (time: number) => void;
  onSessionLost?: (currentTime: number) => void;
  onCapacityBusy?: (retryAfterSec: number) => void;
  /**
   * Reports that a manifest loaded, which is what tells a capacity-busy
   * consumer its stream is no longer queued behind the transcode limiter.
   */
  onManifestLoaded?: () => void;
  /**
   * Reports the profile the server actually ran, which differs from the
   * requested one whenever the remux safety gate forced a transcode.
   */
  onEffectiveProfile?: (profileId: string) => void;
  /** Reports the validated absolute start measured for the HLS media. */
  onActualStart?: (startSec: number) => void;
};

function loadHlsLight() {
  return import("hls.js/light");
}

const HLS_NETWORK_ERROR = "networkError" as ErrorData["type"];
const HLS_MEDIA_ERROR = "mediaError" as ErrorData["type"];

/**
 * hls.js keeps its own retry defaults; only the budgets are ours. The server
 * answers a segment request by long-polling and sending nothing until it has
 * an answer, so time-to-first-byte needs the same budget as the whole load.
 */
function loadPolicyForTimeout(timeoutMs: number) {
  return {
    default: {
      maxTimeToFirstByteMs: timeoutMs,
      maxLoadTimeMs: timeoutMs,
      timeoutRetry: { maxNumRetry: 2, retryDelayMs: 0, maxRetryDelayMs: 0 },
      errorRetry: { maxNumRetry: 4, retryDelayMs: 1000, maxRetryDelayMs: 8000 },
    },
  };
}

type ManifestHeaders = {
  get: (name: string) => string | null;
};

type ManifestMetadata = {
  effectiveProfile: string | null;
  actualStartSec: number | null;
};

function readManifestHeader(
  headers: ManifestHeaders | null,
  name: string,
): string | null {
  if (!headers) return null;

  try {
    return headers.get(name);
  } catch {
    return null;
  }
}

function manifestHeadersFromNetworkDetails(
  networkDetails: unknown,
): ManifestHeaders | null {
  try {
    const xhr = networkDetails as XMLHttpRequest | null | undefined;
    if (typeof xhr?.getResponseHeader !== "function") return null;

    return {
      get: (name) => xhr.getResponseHeader(name),
    };
  } catch {
    return null;
  }
}

function parseManifestMetadata(
  headers: ManifestHeaders | null,
  requestedStartSec: number | undefined,
): ManifestMetadata {
  const effectiveProfile =
    readManifestHeader(headers, HLS_EFFECTIVE_PROFILE_HEADER)?.trim() || null;
  const rawActualStart = readManifestHeader(
    headers,
    HLS_ACTUAL_START_HEADER,
  )?.trim();

  if (
    !rawActualStart ||
    requestedStartSec === undefined ||
    !Number.isFinite(requestedStartSec) ||
    requestedStartSec < 0
  ) {
    return { effectiveProfile, actualStartSec: null };
  }

  const parsedActualStart = Number(rawActualStart);
  const validActualStart =
    Number.isFinite(parsedActualStart) &&
    parsedActualStart >= 0 &&
    parsedActualStart <= requestedStartSec;

  return {
    effectiveProfile,
    actualStartSec: validActualStart ? Math.max(0, parsedActualStart) : null,
  };
}

function retryAfterSecFromHeaders(headers: ManifestHeaders | null): number {
  const header = readManifestHeader(headers, "Retry-After");
  const parsed = header ? Number.parseInt(header, 10) : NaN;
  if (Number.isFinite(parsed) && parsed > 0) {
    return parsed;
  }
  return HLS_CAPACITY_RETRY_FALLBACK_SEC;
}

/**
 * Whether a segment 404 is the end of the media rather than a lost session.
 * The server marks the 404 only when FFmpeg exited cleanly without writing
 * the file: a synthesized transcode playlist can list one or two segments
 * more than FFmpeg produces when a source's audio outlasts its video. The
 * segment index cannot tell the two apart — an unmarked 404 on the last
 * listed segment after an idle eviction is a lost session — so the marker is
 * the only signal used.
 */
function isPastEndSegment(headers: ManifestHeaders | null): boolean {
  return (
    readManifestHeader(headers, HLS_SEGMENT_STATUS_HEADER)?.trim() ===
    HLS_SEGMENT_STATUS_PAST_END
  );
}

export default function VideoPlayer({
  videoRef,
  src,
  isHlsSource,
  title,
  isFullscreen = false,
  onError,
  onPlay,
  onPause,
  onEnded,
  onTimeUpdate,
  onDurationChange,
  onNativeError,
  subtitleTrack = null,
  startSec = 0,
  requestedStartSec,
  onStartApplied,
  onSessionLost,
  onCapacityBusy,
  onManifestLoaded,
  onEffectiveProfile,
  onActualStart,
}: VideoPlayerProps) {
  const hlsRef = useRef<Hls | null>(null);
  // True after a past-the-end segment 404 stopped hls.js loading. Loading has
  // to be re-armed on the next seek, or a jump back past the back buffer
  // never fetches again; a seek into the same tail just re-enters the branch.
  const hlsLoadStoppedAtEndRef = useRef(false);

  const resumeHlsLoadAfterEnd = (video: HTMLVideoElement) => {
    const hls = hlsRef.current;
    if (!hls || !hlsLoadStoppedAtEndRef.current) return;
    hlsLoadStoppedAtEndRef.current = false;
    hls.startLoad(video.currentTime);
  };

  // Mid-playback buffering indicator: shown only after a short delay so
  // sub-perceptual stalls never flash a spinner.
  const [showBuffering, setShowBuffering] = useState(false);
  const bufferingDelayTimerRef = useRef<number | null>(null);

  const clearBufferingIndicator = () => {
    if (bufferingDelayTimerRef.current !== null) {
      window.clearTimeout(bufferingDelayTimerRef.current);
      bufferingDelayTimerRef.current = null;
    }
    setShowBuffering(false);
  };

  const scheduleBufferingIndicator = () => {
    if (bufferingDelayTimerRef.current !== null || showBuffering) return;
    bufferingDelayTimerRef.current = window.setTimeout(() => {
      bufferingDelayTimerRef.current = null;
      setShowBuffering(true);
    }, MOVIE_BUFFERING_SPINNER_DELAY_MS);
  };

  // A source change (e.g. an HLS session rebase) must not inherit a stale
  // spinner or a pending show timer from the previous stream.
  useEffect(() => {
    return () => {
      clearBufferingIndicator();
    };
  }, [src]);

  const reportError = useEffectEvent((message: string) => {
    onError(message);
  });

  const handleStartApplied = useEffectEvent((time: number) => {
    onStartApplied?.(time);
  });

  const applyStartTime = useEffectEvent((video: HTMLVideoElement) => {
    const duration = Number.isFinite(video.duration) ? video.duration : 0;
    const nextTime = duration > 0 ? Math.min(startSec, duration) : startSec;
    video.currentTime = nextTime;
    handleStartApplied(nextTime);
  });

  // Returns false when no onCapacityBusy consumer exists so the caller can
  // fall through to the regular fatal-error handling.
  const handleCapacityBusy = useEffectEvent(
    (headers: ManifestHeaders | null): boolean => {
      if (!onCapacityBusy) return false;

      onCapacityBusy(retryAfterSecFromHeaders(headers));
      return true;
    },
  );

  const reportManifestMetadata = useEffectEvent(
    (headers: ManifestHeaders | null) => {
      const metadata = parseManifestMetadata(headers, requestedStartSec);

      if (onManifestLoaded) {
        onManifestLoaded();
      }
      if (metadata.effectiveProfile && onEffectiveProfile) {
        onEffectiveProfile(metadata.effectiveProfile);
      }
      if (metadata.actualStartSec !== null && onActualStart) {
        onActualStart(metadata.actualStartSec);
      }
    },
  );
  const needsNativeManifestPreflight =
    isHlsSource &&
    (onCapacityBusy !== undefined ||
      onManifestLoaded !== undefined ||
      onEffectiveProfile !== undefined ||
      onActualStart !== undefined ||
      onSessionLost !== undefined);

  // Rate limiting and the max-attempt budget live in useHlsSessionRecovery
  // (the onSessionLost consumer); this component only reports the event.
  // Returns false when no consumer exists so the caller can fall through to
  // the regular fatal-error handling, matching handleCapacityBusy.
  const reportSessionLost = useEffectEvent((currentTime: number): boolean => {
    if (!onSessionLost) return false;

    onSessionLost(currentTime);
    return true;
  });

  // A source that has buffered nothing has no playhead of its own yet:
  // `currentTime` still reads 0, the new session's start. Reporting that sent
  // each recovery to the start of the failed window, which is ten seconds
  // before its target (the resume rewind), so a 404 that kept coming back
  // walked the film backwards. The start this source was built for is the
  // position the viewer was actually at.
  const reportSessionLostAtStart = useEffectEvent((): boolean =>
    reportSessionLost(startSec),
  );

  const handleHlsError = useEffectEvent((data: ErrorData) => {
    const detail = data.details ?? "unknown error";
    if (data.type === HLS_NETWORK_ERROR) {
      reportError(`Network error loading stream (${detail}).`);
    } else if (data.type === HLS_MEDIA_ERROR) {
      reportError(`The browser could not decode this stream (${detail}).`);
    } else {
      reportError(`Stream error: ${detail}`);
    }
  });

  // hls.js source lifecycle. `startSec` stays in the deps deliberately: the
  // playlist URL can stay identical while the resume target changes (any seek
  // inside the rewind buffer keeps start=0 in the URL), and rebuilding with
  // `startPosition` is what applies that seek.
  // The cleanup calls disposeHls() -> hls.destroy(), which removes every listener
  // registered below; the rule does not model destroy().
  // react-doctor-disable-next-line react-doctor/effect-needs-cleanup
  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;
    if (!isHlsSource || prefersNativeHLS) return;

    let cancelled = false;
    let disposeHls: (() => void) | null = null;

    void (async () => {
      try {
        const { default: Hls } = await loadHlsLight();
        if (cancelled) return;
        if (!Hls.isSupported()) {
          // Neither Media Source Extensions nor native HLS: without this the
          // player would sit blank with no explanation at all.
          reportError(
            "This browser cannot play streamed video (no Media Source Extensions support).",
          );
          return;
        }

        const hls = new Hls({
          xhrSetup(xhr: XMLHttpRequest) {
            xhr.withCredentials = true;
          },
          manifestLoadPolicy: loadPolicyForTimeout(HLS_JS_LOAD_TIMEOUT_MS),
          playlistLoadPolicy: loadPolicyForTimeout(HLS_JS_LOAD_TIMEOUT_MS),
          fragLoadPolicy: loadPolicyForTimeout(HLS_JS_FRAG_LOAD_TIMEOUT_MS),
          backBufferLength: HLS_JS_BACK_BUFFER_LENGTH_SEC,
          startPosition: startSec > 0 ? startSec : -1,
        });
        hlsRef.current = hls;
        hlsLoadStoppedAtEndRef.current = false;
        let networkRecoveryTimer: number | undefined;
        disposeHls = () => {
          window.clearTimeout(networkRecoveryTimer);
          hls.destroy();
          // A late dispose must not clobber a newer instance a subsequent
          // effect run already put in the ref.
          if (hlsRef.current === hls) {
            hlsRef.current = null;
          }
        };

        hls.loadSource(src);
        hls.attachMedia(video);

        if (startSec > 0) {
          hls.once(Hls.Events.MANIFEST_PARSED, () => {
            handleStartApplied(startSec);
          });
        }

        // The manifest response names the profile the server really ran. The
        // request URL cannot: a remux request that fails the safety gate is
        // still served from the /hls/remux/ path.
        hls.once(Hls.Events.MANIFEST_LOADED, (_event, data) => {
          reportManifestMetadata(
            manifestHeadersFromNetworkDetails(data.networkDetails),
          );
        });

        // Every way the server reports that a session no longer exists. It
        // 404s the playlist handler as well as the segment handler, and hls.js
        // surfaces those as manifest/level errors — matching only the fragment
        // detail sent them to the error screen instead of into recovery.
        const sessionLostDetails: ErrorData["details"][] = [
          Hls.ErrorDetails.FRAG_LOAD_ERROR,
          Hls.ErrorDetails.MANIFEST_LOAD_ERROR,
          Hls.ErrorDetails.LEVEL_LOAD_ERROR,
        ];
        const playlistDetails: ErrorData["details"][] = [
          Hls.ErrorDetails.MANIFEST_LOAD_ERROR,
          Hls.ErrorDetails.LEVEL_LOAD_ERROR,
        ];

        let mediaRecoveryAttempted = false;
        let networkRecoveryAttempts = 0;
        let segmentNotReadyRetries = 0;
        let fragmentBuffered = false;
        // One report per instance: a lost session fails the requests that
        // are still in flight too, and the page replaces this instance after
        // the first report. Reporting each of them used to spend the
        // recovery budget on a single failure.
        let sessionLostReported = false;
        const reportLost = (): boolean => {
          if (sessionLostReported) return true;
          const reported = fragmentBuffered
            ? reportSessionLost(video.currentTime)
            : reportSessionLostAtStart();
          sessionLostReported = reported;
          return reported;
        };

        // A fragment that buffers proves the stream recovered, so the one-shot
        // budgets above are for consecutive failures rather than for the life
        // of the instance: a session that stalls once an hour can recover each
        // time instead of dying on the second incident.
        hls.on(Hls.Events.FRAG_BUFFERED, () => {
          fragmentBuffered = true;
          mediaRecoveryAttempted = false;
          networkRecoveryAttempts = 0;
          segmentNotReadyRetries = 0;
        });

        hls.on(Hls.Events.ERROR, (_event: Events.ERROR, data: ErrorData) => {
          const responseCode = data.response?.code;

          // Checked ahead of the session-lost rule below, which would
          // otherwise rebase the session three times over a segment that
          // cannot exist and then report the film as unrecoverable. hls.js
          // only signals end of stream after buffering the last listed
          // segment, so the player asks for it directly.
          if (
            responseCode === 404 &&
            data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR &&
            isPastEndSegment(
              manifestHeadersFromNetworkDetails(data.networkDetails),
            )
          ) {
            hlsLoadStoppedAtEndRef.current = true;
            hls.stopLoad();
            hls.trigger(Hls.Events.BUFFER_EOS, { type: null });
            return;
          }

          if (
            responseCode === 404 &&
            sessionLostDetails.includes(data.details) &&
            reportLost()
          ) {
            return;
          }

          if (
            data.fatal &&
            responseCode === 503 &&
            playlistDetails.includes(data.details) &&
            handleCapacityBusy(
              manifestHeadersFromNetworkDetails(data.networkDetails),
            )
          ) {
            return;
          }

          if (!data.fatal) return;

          // 503 on a fragment is not a failure: the server long-polled and the
          // encoder still had not reached that segment. hls.js has exhausted
          // its own retries by the time this is fatal, so give it a bounded
          // number of fresh attempts before calling the stream dead.
          if (
            responseCode === 503 &&
            data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR
          ) {
            if (segmentNotReadyRetries < HLS_SEGMENT_NOT_READY_MAX_RETRIES) {
              segmentNotReadyRetries += 1;
              hls.startLoad();
              return;
            }
            reportError(
              "The server is still preparing this part of the video. Try again in a moment.",
            );
            return;
          }

          // A 500 on a segment means FFmpeg died partway through. The server
          // replaces a failed session on the next manifest request, so a
          // rebase at the playhead gets a working one; the recovery budget
          // stops a failure that repeats at the same point.
          if (
            responseCode === 500 &&
            data.details === Hls.ErrorDetails.FRAG_LOAD_ERROR &&
            reportLost()
          ) {
            return;
          }

          if (data.type === HLS_MEDIA_ERROR && !mediaRecoveryAttempted) {
            mediaRecoveryAttempted = true;
            hls.recoverMediaError();
            return;
          }

          // hls.js's documented recovery for a fatal network error. Only for
          // requests that never got an answer, meaning a timeout or a dropped
          // connection. A status code means the server decided something, and
          // the cases worth retrying are the ones handled above; blindly
          // reloading the rest would just hide a definite failure behind a
          // second identical response. A timeout carries no response, but a
          // dropped connection reports status 0 (the XHR loader passes
          // xhr.status through), and hls.js never retries a 0 while the
          // browser thinks it is online. So a server restart used to land
          // straight on the error screen. The backoff gives a restarting
          // server time to come back, and once it has, the next request 404s
          // and the session-lost rule rebases.
          if (
            data.type === HLS_NETWORK_ERROR &&
            (responseCode === undefined || responseCode === 0) &&
            networkRecoveryAttempts < HLS_NETWORK_RECOVERY_DELAYS_MS.length
          ) {
            const delayMs =
              HLS_NETWORK_RECOVERY_DELAYS_MS[networkRecoveryAttempts];
            networkRecoveryAttempts += 1;
            window.clearTimeout(networkRecoveryTimer);
            networkRecoveryTimer = window.setTimeout(() => {
              // Nothing is loaded without a manifest, so startLoad() has
              // nothing to resume; fetching the source again does.
              if (data.details === Hls.ErrorDetails.MANIFEST_LOAD_ERROR) {
                hls.loadSource(src);
              } else {
                hls.startLoad();
              }
            }, delayMs);
            return;
          }

          handleHlsError(data);
        });
      } catch {
        if (!cancelled) {
          reportError("Failed to load the video playback engine.");
        }
      }
    })();

    return () => {
      cancelled = true;
      disposeHls?.();
    };
  }, [isHlsSource, src, startSec, videoRef]);

  // Native source lifecycle: direct play, and built-in HLS on browsers without
  // Media Source Extensions (iPhone Safari). No
  // `startSec` dep — the direct URL is a constant, so a start change must
  // seek (the effect below) rather than tear down the source and refetch
  // from byte 0 (audit D10). Native HLS start changes arrive as a new `src`.
  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;
    if (isHlsSource && !prefersNativeHLS) return;

    const clearSource = () => {
      video.removeAttribute("src");
      video.load();
    };
    if (!needsNativeManifestPreflight) {
      video.src = src;
      return clearSource;
    }

    let cancelled = false;
    const controller = new AbortController();
    // Cleared in both exits below; `finally` would bail React Compiler out of
    // the whole component.
    let timeoutId: number | undefined;

    void (async () => {
      try {
        // React Strict Mode immediately cleans up and re-runs effects once in
        // development. Defer the request until that probe mount can cancel so
        // it cannot issue a duplicate manifest preflight.
        await Promise.resolve();
        if (cancelled) return;

        // A stalled manifest connection would otherwise leave native playback
        // sourceless forever; aborting drops into the catch fallback below.
        timeoutId = window.setTimeout(
          () => controller.abort(),
          HLS_JS_LOAD_TIMEOUT_MS,
        );
        const response = await fetch(src, {
          credentials: "include",
          signal: controller.signal,
        });
        window.clearTimeout(timeoutId);
        await releaseResponseBody(response);
        if (cancelled) return;

        const handledCapacityFailure =
          response.status === 503 && handleCapacityBusy(response.headers);
        if (handledCapacityFailure) return;

        // A 404 manifest is a dead session, the same signal the hls.js error
        // handler routes to onSessionLost. Handing it to the native player
        // instead would surface only a generic media error.
        const handledLostSession =
          response.status === 404 && reportSessionLostAtStart();
        if (handledLostSession) return;

        if (response.ok) {
          reportManifestMetadata(response.headers);
        }
        if (!cancelled) {
          video.src = src;
        }
      } catch {
        window.clearTimeout(timeoutId);
        if (!cancelled) {
          // Metadata is advisory. Let the native player use its existing
          // loading and error behavior when the preflight itself fails.
          video.src = src;
        }
      }
    })();

    return () => {
      cancelled = true;
      controller.abort();
      clearSource();
    };
  }, [isHlsSource, needsNativeManifestPreflight, src, videoRef]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || startSec <= 0) return;
    // HLS.js owns the initial seek via its startPosition config and fires
    // onStartApplied via MANIFEST_PARSED; don't compete with it. Gated on
    // source type because hlsRef is assigned asynchronously and is still
    // null when this effect runs on a fresh hls.js mount.
    if (isHlsSource && !prefersNativeHLS) return;
    if (hlsRef.current) return;

    if (video.readyState >= 1) {
      applyStartTime(video);
      return;
    }

    const handleLoadedMetadata = () => {
      applyStartTime(video);
    };

    video.addEventListener("loadedmetadata", handleLoadedMetadata, {
      once: true,
    });
    return () => {
      video.removeEventListener("loadedmetadata", handleLoadedMetadata);
    };
  }, [isHlsSource, startSec, src, videoRef]);

  // The subtitleTrack object gets a new reference on every parent render;
  // key on the URL which uniquely identifies the active subtitle so the
  // effect only re-runs when the subtitle actually changes.
  const subtitleUrl = subtitleTrack?.url ?? null;
  const subtitleTrackRef = useRef(subtitleTrack);
  useEffect(() => {
    subtitleTrackRef.current = subtitleTrack;
  }, [subtitleTrack]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const existing = video.querySelector("track[data-subtitle]");
    if (existing) {
      video.removeChild(existing);
    }

    const sub = subtitleTrackRef.current;
    if (!sub) return;

    const track = document.createElement("track");
    track.kind = "subtitles";
    track.src = sub.url;
    track.srclang = sub.srclang;
    track.label = sub.label;
    track.setAttribute("data-subtitle", "");
    video.appendChild(track);
    track.track.mode = "showing";

    return () => {
      if (video.contains(track)) {
        video.removeChild(track);
      }
    };
  }, [subtitleUrl, videoRef]);

  return (
    <div
      className={
        isFullscreen
          ? "relative flex min-h-0 w-full flex-1 items-center justify-center bg-black"
          : "relative flex min-h-0 w-full flex-1 items-center justify-center p-4"
      }
    >
      <div
        className={
          isFullscreen
            ? "size-full min-h-0 min-w-0"
            : "aspect-video w-full max-w-6xl"
        }
      >
        {/* Subtitle tracks are attached imperatively below, keyed on subtitleUrl. */}
        {/* react-doctor-disable-next-line react-doctor/media-has-caption */}
        <video
          ref={videoRef}
          className={`size-full bg-black object-contain ${isFullscreen ? "rounded-none" : "rounded-lg"}`}
          playsInline
          aria-label={`Video player for ${title}`}
          onPlay={onPlay}
          onPause={() => {
            clearBufferingIndicator();
            onPause?.();
          }}
          onEnded={() => {
            clearBufferingIndicator();
            onEnded?.();
          }}
          onWaiting={scheduleBufferingIndicator}
          onStalled={scheduleBufferingIndicator}
          onSeeking={(e) => {
            scheduleBufferingIndicator();
            resumeHlsLoadAfterEnd(e.currentTarget);
          }}
          onPlaying={clearBufferingIndicator}
          onCanPlay={clearBufferingIndicator}
          onSeeked={clearBufferingIndicator}
          onTimeUpdate={
            onTimeUpdate
              ? (e) => onTimeUpdate(e.currentTarget.currentTime)
              : undefined
          }
          onDurationChange={
            onDurationChange
              ? (e) => onDurationChange(e.currentTarget.duration)
              : undefined
          }
          onError={(e) => {
            clearBufferingIndicator();

            const errorCode = e.currentTarget.error?.code;

            onNativeError?.(errorCode);

            switch (errorCode) {
              case MediaError.MEDIA_ERR_ABORTED:
                onError(
                  "Playback was interrupted before the stream finished loading.",
                );
                return;
              case MediaError.MEDIA_ERR_NETWORK:
                onError("A network error interrupted video playback.");
                return;
              case MediaError.MEDIA_ERR_DECODE:
                onError("The browser could not decode this video stream.");
                return;
              case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED:
                onError(
                  "This video format or stream is not supported by the browser.",
                );
                return;
              default:
                onError(
                  "Playback failed — the browser could not play this stream.",
                );
            }
          }}
        />
      </div>
      {showBuffering && (
        <div
          className={cn(
            MOTION_MEDIA_OVERLAY_ENTER_CLASS,
            "pointer-events-none absolute inset-0 z-10 flex items-center justify-center",
          )}
        >
          <div className="flex size-16 items-center justify-center rounded-full bg-background/80 backdrop-blur-sm">
            <Spinner className="size-8 text-primary" aria-label="Buffering" />
          </div>
        </div>
      )}
    </div>
  );
}
