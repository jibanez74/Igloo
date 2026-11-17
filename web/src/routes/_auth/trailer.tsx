import {
  useRef,
  useEffect,
  useEffectEvent,
  useState,
  type ComponentType,
  type ReactNode,
  type RefObject,
} from "react";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  AlertCircle,
  ArrowLeft,
  Film,
  X,
  Rewind,
  FastForward,
  Pause,
  Play,
  Maximize,
  Minimize,
  RotateCcw,
  type LucideProps,
} from "lucide-react";
import ProgressBar from "@/components/playback/ProgressBar";
import { VolumeLevelIcon } from "@/components/playback/VolumeControl";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogDescription,
  DialogFullscreenContent,
  DialogTitle,
} from "@/components/ui/dialog";
import { movieDetailsQueryOpts } from "@/lib/query-opts";
import { trailerSearchSchema } from "@/lib/route-search";
import { routeHead } from "@/lib/route-head";
import { useYouTubePlayer } from "@/hooks/useYouTubePlayer";
import { useSuspendMusicPlayback } from "@/hooks/useSuspendMusicPlayback";
import { useShortcutHints } from "@/hooks/useShortcutHints";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { useVideoPlaybackKeyboard } from "@/hooks/useVideoPlaybackKeyboard";
import { formatTimecode } from "@/lib/format";
import {
  MOVIE_SEEK_STEP_SEC,
  MOVIE_VOLUME_STEP,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
} from "@/lib/constants";
import {
  canRequestElementFullscreen,
  exitDocumentFullscreen,
  getFullscreenElement,
  isDocumentFullscreenEntryLikely,
  requestElementFullscreen,
} from "@/lib/fullscreen";
import { cn } from "@/lib/utils";
import { showInfo } from "@/lib/toast-helpers";
import { buildYouTubeThumbnailUrl } from "@/lib/youtube-thumb-url";

const TRAILER_FALLBACK_HEAD = routeHead("Trailer");

export const Route = createFileRoute("/_auth/trailer")({
  // The player fills the window: no sidebar, header or page padding.
  staticData: { immersive: true },
  validateSearch: trailerSearchSchema,
  loaderDeps: ({ search }) => ({
    mediaType: search.mediaType,
    tmdbId: search.tmdbId,
    videoKey: search.videoKey,
  }),
  loader: async ({ context, deps }) => {
    // Titled only when the page itself shows this movie's trailer, so every
    // other case skips the movie lookup the page would not make either.
    if (
      deps.mediaType !== "movie" ||
      !deps.tmdbId ||
      deps.tmdbId <= 0 ||
      deps.videoKey
    ) {
      return { movieTitle: null };
    }

    const res = await context.queryClient.ensureQueryData(
      movieDetailsQueryOpts(deps.tmdbId),
    );

    return {
      movieTitle: !res.error ? (res.data.movie?.title ?? null) : null,
    };
  },
  head: ({ loaderData }) =>
    loaderData?.movieTitle
      ? routeHead(`${loaderData.movieTitle} - Trailer`)
      : TRAILER_FALLBACK_HEAD,
  component: TrailerPage,
});

function handleDialogEscapeKeyDown(event: KeyboardEvent) {
  if (!getFullscreenElement()) {
    return;
  }

  event.preventDefault();
  void exitDocumentFullscreen();
}

/** The level an Unmute at volume 0 restores, on YouTube's 0–100 scale. */
const UNMUTE_DEFAULT_VOLUME = 50;

type DialogView =
  | "playback-error"
  | "loading"
  | "load-error"
  | "no-trailer"
  | "player";

function TrailerPage() {
  const { mediaType, tmdbId, videoKey, returnTo } = Route.useSearch();
  const navigate = Route.useNavigate();
  const router = useRouter();
  useSuspendMusicPlayback();
  const { showShortcutHints, withShortcut } = useShortcutHints();

  const containerRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const [isBrowserFullscreen, setIsBrowserFullscreen] = useState(
    () => !!getFullscreenElement(),
  );

  const shouldFetchMovie =
    mediaType === "movie" && tmdbId != null && tmdbId > 0 && !videoKey;
  const {
    data,
    isPending: moviePending,
    isError: movieIsError,
    refetch: refetchMovie,
  } = useQuery({
    ...movieDetailsQueryOpts(tmdbId ?? 0),
    enabled: shouldFetchMovie,
  });

  const media = data?.data?.movie;
  const trailerFromApi = media?.videos?.results?.find(
    v => v.type === "Trailer" && v.site === "YouTube",
  );
  const trailerKey = videoKey ?? trailerFromApi?.key ?? null;

  const title = media?.title ? `${media.title} - Trailer` : "Trailer";

  const handleClose = async () => {
    try {
      await router.navigate({ to: returnTo ?? "/" });
    } catch {
      await navigate({ to: "/" });
    }
    // After the navigation settles and the dialog (with its focus trap) has
    // gone; focused any earlier, a slow destination loader left the trap in
    // place to pull focus back, and focus then fell to the body.
    window.setTimeout(() => {
      document.getElementById("main")?.focus({ preventScroll: true });
    }, 0);
  };
  const close = () => void handleClose();

  // YouTube's own controls stay off: Igloo's footer is the one set of
  // controls, and the click catcher over the frame keeps focus (and so the
  // shortcuts and Escape) in this document rather than in the iframe.
  const {
    containerRef: playerContainerRef,
    isReady,
    isPlaying,
    currentTime,
    duration,
    volume,
    isMuted,
    error,
    play,
    pause,
    togglePlay,
    seekTo,
    seekForward,
    seekBackward,
    setVolume,
    toggleMute,
    retry,
  } = useYouTubePlayer({
    videoId: trailerKey,
    autoplay: true,
    controls: false,
    onEnd: close,
  });

  const dialogView: DialogView = error
    ? "playback-error"
    : !trailerKey && shouldFetchMovie && moviePending
      ? "loading"
      : !trailerKey && (movieIsError || data?.error)
        ? "load-error"
        : !trailerKey
          ? "no-trailer"
          : "player";

  useEffect(() => {
    const onFullscreenChange = () => {
      setIsBrowserFullscreen(!!getFullscreenElement());
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

  // The dialog already fills the window, so a browser without element
  // fullscreen has no expanded view to fall back to, only this notice.
  const toggleFullscreen = () => {
    const el = containerRef.current;
    if (!el) return;

    if (getFullscreenElement()) {
      void exitDocumentFullscreen();
      return;
    }

    if (
      !canRequestElementFullscreen(el) ||
      !isDocumentFullscreenEntryLikely()
    ) {
      showInfo("Full screen isn't available in this browser.");
      return;
    }

    void requestElementFullscreen(el).catch(() => {
      showInfo("Full screen isn't available in this browser.");
    });
  };

  // At volume 0 the mute flag alone brings nothing back, so unmuting there
  // also restores a level.
  const toggleAudio = () => {
    if (volume === 0) {
      setVolume(UNMUTE_DEFAULT_VOLUME);
      return;
    }
    toggleMute();
  };

  const seekBack = () => seekBackward(MOVIE_SEEK_STEP_SEC);
  const seekAhead = () => seekForward(MOVIE_SEEK_STEP_SEC);

  // Escape stays with the dialog, which closes it or leaves fullscreen.
  useVideoPlaybackKeyboard({
    containerRef,
    playerReady: isReady,
    enabled: dialogView === "player",
    volumeStep: MOVIE_VOLUME_STEP,
    onTogglePlay: togglePlay,
    onSeekBackward: seekBack,
    onSeekForward: seekAhead,
    onSeekToStart: () => seekTo(0),
    onAdjustVolume: (delta) => setVolume(volume + delta * 100),
    onToggleMute: toggleAudio,
    onToggleFullscreen: toggleFullscreen,
  });

  // The trailer takes the OS media controls from the paused music, which a
  // headset Play would otherwise resume over it.
  useVideoMediaSession({
    title,
    artworkUrl: buildYouTubeThumbnailUrl(trailerKey) || null,
    currentTime,
    duration,
    playing: isPlaying,
    seekStepSec: MOVIE_SEEK_STEP_SEC,
    onPlay: play,
    onPause: pause,
    onSeek: seekTo,
    enabled: dialogView === "player",
  });

  const closeLabel = withShortcut("Close trailer", "Escape");

  const focusPrimaryControl = () => {
    const focusTarget = closeButtonRef.current ?? containerRef.current;
    focusTarget?.focus({ preventScroll: true });
  };

  const handleDialogOpenAutoFocus = (event: Event) => {
    event.preventDefault();
    focusPrimaryControl();
  };

  // Every view renders into the same dialog, so Radix's open auto-focus runs
  // for the first view only. A later swap (the player failing, a retry, the
  // movie finishing loading) unmounts the focused control and would strand
  // focus on the dialog container, so each new view takes focus the same way.
  const focusPrimaryControlOnViewChange = useEffectEvent(focusPrimaryControl);
  const previousDialogViewRef = useRef(dialogView);
  useEffect(() => {
    if (previousDialogViewRef.current === dialogView) return;
    previousDialogViewRef.current = dialogView;
    focusPrimaryControlOnViewChange();
  }, [dialogView]);

  const dialogProps = {
    containerRef,
    title,
    closeLabel,
    onClose: close,
    onOpenAutoFocus: handleDialogOpenAutoFocus,
  };
  const goBack = {
    label: "Go Back",
    icon: ArrowLeft,
    onClick: close,
  };

  switch (dialogView) {
    case "playback-error":
      return (
        <TrailerDialog {...dialogProps}>
          <TrailerMessage
            icon={AlertCircle}
            tone="destructive"
            heading="Unable to Play Trailer"
            message={error}
            primaryRef={closeButtonRef}
            primary={{ label: "Try Again", icon: RotateCcw, onClick: retry }}
            secondary={goBack}
          />
        </TrailerDialog>
      );
    case "loading":
      return (
        <TrailerDialog {...dialogProps}>
          <div className="flex flex-1 items-center justify-center p-4">
            <DialogTitle className="sr-only">Loading trailer</DialogTitle>
            <DialogDescription className="sr-only">
              Please wait while the trailer loads.
            </DialogDescription>

            <div className="text-center">
              <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-primary/10">
                <Spinner size="2xl" tone="primary" />
              </div>
              <p className="text-lg font-medium text-foreground">Loading trailer...</p>
              <p className="mt-2 text-sm text-muted-foreground">Please wait</p>
            </div>
          </div>
        </TrailerDialog>
      );
    case "load-error":
      return (
        <TrailerDialog {...dialogProps}>
          <TrailerMessage
            icon={AlertCircle}
            tone="destructive"
            heading="Unable to Load Trailer"
            // The UI owns these words (design-system §3.4): the server
            // answers a lowercase constant, which is not copy.
            message={
              data?.error && data.status === 404
                ? "We couldn’t find that movie on TMDB."
                : "Couldn’t load the trailer details from TMDB."
            }
            primaryRef={closeButtonRef}
            primary={{
              label: "Try Again",
              icon: RotateCcw,
              onClick: () => void refetchMovie(),
            }}
            secondary={goBack}
          />
        </TrailerDialog>
      );
    case "no-trailer":
      return (
        <TrailerDialog {...dialogProps}>
          <TrailerMessage
            icon={Film}
            tone="muted"
            heading="No Trailer Available"
            message="This movie doesn't have a trailer yet."
            primaryRef={closeButtonRef}
            primary={goBack}
          />
        </TrailerDialog>
      );
    case "player":
      break;
  }

  const isEffectivelyMuted = isMuted || volume === 0;

  return (
    <TrailerDialog
      {...dialogProps}
      titleIsDialogTitle
      subtitle="Now Playing"
      closeRef={closeButtonRef}
      // Without the shortcut list there is nothing to describe; saying so
      // keeps Radix from warning about a missing description.
      describedByShortcuts={showShortcutHints}
      before={
        <>
          <LiveAnnouncer
            message={isPlaying ? `Playing: ${title}` : `Paused: ${title}`}
          />
          {showShortcutHints && (
            <DialogDescription className="sr-only">
              Keyboard shortcuts: Space or K to play/pause, J or left arrow to
              rewind {MOVIE_SEEK_STEP_SEC} seconds, L or right arrow to forward{" "}
              {MOVIE_SEEK_STEP_SEC} seconds, up/down arrows for volume, M to
              mute, F for fullscreen, Escape to exit fullscreen or close.
            </DialogDescription>
          )}
        </>
      }
    >
      <div className="relative flex min-h-0 flex-1 items-center justify-center p-4">
        {/* Bounded by the height as well, so the controls below stay on a
            short window instead of being pushed off its bottom. */}
        <div className="relative aspect-video max-h-full w-full max-w-6xl bg-black">
          {/* Inert: the embed's iframe is not a Tab stop, so focus, the
              shortcuts and Escape never leave this document for it. */}
          <div ref={playerContainerRef} className="size-full" inert />
          {/*
            A pointer convenience over the embedded player: toggling here keeps
            focus and the keyboard in this document instead of the YouTube
            iframe. The footer play button is the keyboard route.
          */}
          {/* react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions */}
          <div
            aria-hidden="true"
            className="absolute inset-0"
            onClick={togglePlay}
          />
        </div>

        {!isReady && (
          <div
            role="status"
            className={cn(
              MOTION_MEDIA_OVERLAY_ENTER_CLASS,
              "absolute inset-0 flex items-center justify-center bg-background/90 backdrop-blur-sm",
            )}
          >
            <div className="text-center">
              <div className="mx-auto mb-4 flex size-16 items-center justify-center rounded-full bg-primary/10">
                <Spinner size="xl" tone="primary" />
              </div>
              <p className="font-medium text-foreground">Loading trailer...</p>
            </div>
          </div>
        )}
      </div>

      <footer
        className={cn(
          MOTION_PLAYER_CHROME_PANEL_CLASS,
          "border-t border-border/50 bg-card/95 p-4 backdrop-blur-lg",
        )}
      >
        <div className="mx-auto max-w-4xl">
          <ProgressBar
            variant="trailer"
            currentTime={currentTime}
            duration={duration}
            onSeek={seekTo}
            ariaLabel="Seek through trailer"
          />

          <div className="flex items-center justify-between">
            <div className="flex min-w-[100px] items-center gap-2">
              <span className="text-sm text-muted-foreground tabular-nums">
                {formatTimecode(currentTime, {
                  forceHours: duration >= 3600,
                })}
              </span>
              <span className="text-muted-foreground">/</span>
              <span className="text-sm text-muted-foreground tabular-nums">
                {formatTimecode(duration)}
              </span>
            </div>

            <div
              className="flex items-center gap-2"
              role="group"
              aria-label="Playback controls"
            >
              <button
                type="button"
                onClick={seekBack}
                className={cn(
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-muted",
                )}
                aria-label={withShortcut(
                  `Rewind ${MOVIE_SEEK_STEP_SEC} seconds`,
                  "J or Left Arrow",
                )}
              >
                <Rewind className="size-5" aria-hidden="true" />
              </button>

              <button
                type="button"
                onClick={togglePlay}
                className={cn(
                  PLAYER_PRIMARY_BUTTON_CLASS,
                  "size-14 shadow-glow",
                )}
                aria-label={withShortcut(
                  isPlaying ? "Pause" : "Play",
                  "Space or K",
                )}
              >
                {isPlaying ? (
                  <Pause className="size-6 fill-current" aria-hidden="true" />
                ) : (
                  <Play className="size-6 fill-current" aria-hidden="true" />
                )}
              </button>

              <button
                type="button"
                onClick={seekAhead}
                className={cn(
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-muted",
                )}
                aria-label={withShortcut(
                  `Forward ${MOVIE_SEEK_STEP_SEC} seconds`,
                  "L or Right Arrow",
                )}
              >
                <FastForward className="size-5" aria-hidden="true" />
              </button>
            </div>

            <div className="flex min-w-[100px] items-center justify-end gap-2">
              <button
                type="button"
                onClick={toggleAudio}
                className={cn(
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-muted",
                )}
                aria-label={withShortcut(
                  isEffectivelyMuted ? "Unmute" : "Mute",
                  "M",
                )}
                aria-pressed={isEffectivelyMuted}
              >
                <VolumeLevelIcon
                  level={volume / 100}
                  muted={isMuted}
                  className="size-5"
                />
              </button>

              <button
                type="button"
                onClick={toggleFullscreen}
                className={cn(
                  PLAYER_ICON_BUTTON_CLASS,
                  "size-10 hover:bg-muted",
                )}
                aria-label={withShortcut(
                  isBrowserFullscreen ? "Exit fullscreen" : "Fullscreen",
                  "F",
                )}
                aria-pressed={isBrowserFullscreen}
              >
                {isBrowserFullscreen ? (
                  <Minimize className="size-5" aria-hidden="true" />
                ) : (
                  <Maximize className="size-5" aria-hidden="true" />
                )}
              </button>
            </div>
          </div>
        </div>
      </footer>
    </TrailerDialog>
  );
}

type TrailerDialogProps = {
  containerRef: RefObject<HTMLDivElement | null>;
  title: string;
  closeLabel: string;
  onClose: () => void;
  onOpenAutoFocus: (event: Event) => void;
  titleIsDialogTitle?: boolean;
  subtitle?: string;
  closeRef?: RefObject<HTMLButtonElement | null>;
  /** False when the view has no description at all (see the player view). */
  describedByShortcuts?: boolean;
  /** Rendered ahead of the header: announcers and the sr-only description. */
  before?: ReactNode;
  children: ReactNode;
};

// The one dialog every view renders into, so the way out never moves between
// loading, an error, a missing trailer and the player itself.
function TrailerDialog({
  containerRef,
  title,
  closeLabel,
  onClose,
  onOpenAutoFocus,
  titleIsDialogTitle = false,
  subtitle,
  closeRef,
  describedByShortcuts = true,
  before,
  children,
}: TrailerDialogProps) {
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogFullscreenContent
        ref={containerRef}
        className={cn(
          MOTION_MEDIA_OVERLAY_ENTER_CLASS,
          "flex flex-col bg-linear-to-b from-card via-background to-card",
        )}
        onOpenAutoFocus={onOpenAutoFocus}
        onEscapeKeyDown={handleDialogEscapeKeyDown}
        {...(describedByShortcuts ? {} : { "aria-describedby": undefined })}
      >
        {before}
        <TrailerHeader
          title={title}
          titleIsDialogTitle={titleIsDialogTitle}
          subtitle={subtitle}
          closeLabel={closeLabel}
          closeRef={closeRef}
          onClose={onClose}
        />
        {children}
      </DialogFullscreenContent>
    </Dialog>
  );
}

type TrailerAction = {
  label: string;
  icon: ComponentType<LucideProps>;
  onClick: () => void;
};

type TrailerMessageProps = {
  icon: ComponentType<LucideProps>;
  tone: "destructive" | "muted";
  heading: string;
  message: string | null;
  /** The view's first control, which takes focus when the view appears. */
  primaryRef: RefObject<HTMLButtonElement | null>;
  primary: TrailerAction;
  secondary?: TrailerAction;
};

const MESSAGE_TONE_CLASSES = {
  destructive: { badge: "bg-destructive/10", icon: "text-destructive" },
  muted: { badge: "bg-muted", icon: "text-muted-foreground" },
} as const;

// A view that explains why there is no trailer and offers the ways on.
function TrailerMessage({
  icon: Icon,
  tone,
  heading,
  message,
  primaryRef,
  primary,
  secondary,
}: TrailerMessageProps) {
  const toneClasses = MESSAGE_TONE_CLASSES[tone];
  const PrimaryIcon = primary.icon;

  return (
    <div className="flex flex-1 items-center justify-center p-4">
      <div className="max-w-md px-4 text-center">
        <div
          className={cn(
            "mx-auto mb-6 flex size-20 items-center justify-center rounded-full",
            toneClasses.badge,
          )}
        >
          <Icon className={cn("size-10", toneClasses.icon)} aria-hidden="true" />
        </div>
        <DialogTitle className="mb-2 text-xl">{heading}</DialogTitle>
        <DialogDescription className="mb-6">{message}</DialogDescription>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Button
            ref={primaryRef}
            onClick={primary.onClick}
            size="xl"
            shape="pill"
            className="font-semibold shadow-glow"
          >
            <PrimaryIcon className="size-4" aria-hidden="true" />
            {primary.label}
          </Button>
          {secondary && (
            <Button
              onClick={secondary.onClick}
              variant="outline"
              size="xl"
              shape="pill"
              className="font-semibold"
            >
              <secondary.icon className="size-4" aria-hidden="true" />
              {secondary.label}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

type TrailerHeaderProps = {
  title: string;
  /** The player view names the dialog from the header; other views name it
   *  from their own heading and show the title as plain text here. */
  titleIsDialogTitle: boolean;
  /** "Now Playing" under the player; the other views have no second line,
   *  since without a movie the title is already just "Trailer". */
  subtitle?: string;
  closeLabel: string;
  closeRef?: RefObject<HTMLButtonElement | null>;
  onClose: () => void;
};

// Every trailer view wears the same header, so the way out never moves
// between loading, an error, a missing trailer and the player itself.
function TrailerHeader({
  title,
  titleIsDialogTitle,
  subtitle,
  closeLabel,
  closeRef,
  onClose,
}: TrailerHeaderProps) {
  return (
    <header
      className={cn(
        MOTION_PLAYER_CHROME_PANEL_CLASS,
        "flex items-center justify-between border-b border-border/50 bg-card/95 px-4 py-3 backdrop-blur-lg",
      )}
    >
      <div className="flex min-w-0 items-center gap-3">
        <Film className="size-5 shrink-0 text-primary" aria-hidden="true" />
        <div className="min-w-0">
          {titleIsDialogTitle ? (
            <DialogTitle className="truncate text-base">
              {title}
            </DialogTitle>
          ) : (
            <p className="truncate text-base font-semibold text-foreground">
              {title}
            </p>
          )}
          {subtitle && (
            <p className="text-xs text-muted-foreground">{subtitle}</p>
          )}
        </div>
      </div>
      <button
        type="button"
        ref={closeRef}
        onClick={onClose}
        className={cn(PLAYER_ICON_BUTTON_CLASS, "size-10 shrink-0 hover:bg-muted")}
        aria-label={closeLabel}
      >
        <X className="size-5" aria-hidden="true" />
      </button>
    </header>
  );
}
