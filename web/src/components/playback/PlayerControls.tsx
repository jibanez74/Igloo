import type { RefObject } from "react";
import {
  FastForward,
  Maximize,
  Minimize,
  Pause,
  Play,
  Rewind,
} from "lucide-react";
import ProgressBar from "@/components/playback/ProgressBar";
import ChapterMenu from "@/components/playback/ChapterMenu";
import VolumeControl from "@/components/playback/VolumeControl";
import { useShortcutHints } from "@/hooks/useShortcutHints";
import {
  MOVIE_SEEK_STEP_SEC,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
  PLAYER_PRIMARY_BUTTON_CLASS,
} from "@/lib/constants";
import { formatTimecode } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { PlaybackChapterType } from "@/types";

type PlayerControlsProps = {
  chromeFullscreenMode: boolean;
  controlsVisible: boolean;
  isFullscreen: boolean;
  isImmersiveViewport: boolean;
  currentTime: number;
  duration: number;
  displayedDuration: number;
  playing: boolean;
  modeLabel: string;
  chapters: PlaybackChapterType[];
  videoRef: RefObject<HTMLVideoElement | null>;
  onSeek: (time: number) => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onTogglePlay: () => void;
  onToggleFullscreen: () => void;
  onSelectChapter: (startTimeSec: number, title: string) => void;
};

export default function PlayerControls({
  chromeFullscreenMode,
  controlsVisible,
  isFullscreen,
  isImmersiveViewport,
  currentTime,
  duration,
  displayedDuration,
  playing,
  modeLabel,
  chapters,
  videoRef,
  onSeek,
  onSeekBackward,
  onSeekForward,
  onTogglePlay,
  onToggleFullscreen,
  onSelectChapter,
}: PlayerControlsProps) {
  const { withShortcut } = useShortcutHints();

  const exitFullscreenLabel =
    isImmersiveViewport && !isFullscreen
      ? "Exit expanded view"
      : "Exit fullscreen";
  const fullscreenLabel = chromeFullscreenMode
    ? exitFullscreenLabel
    : "Fullscreen";

  return (
    <footer
      className={
        chromeFullscreenMode
          ? cn(
              MOTION_PLAYER_CHROME_PANEL_CLASS,
              "absolute inset-x-0 bottom-0 z-10 border-t border-border bg-background/95 p-4 backdrop-blur-lg",
              controlsVisible
                ? "translate-y-0 opacity-100"
                : "pointer-events-none translate-y-full opacity-0",
            )
          : "shrink-0 border-t border-border bg-background/95 p-4 backdrop-blur-lg"
      }
    >
      <div className="mx-auto max-w-4xl">
        <div className="mb-4" role="group" aria-label="Playback progress">
          <ProgressBar
            variant="video"
            currentTime={currentTime}
            duration={duration}
            displayedDuration={displayedDuration}
            onSeek={onSeek}
          />
        </div>

        <div className="flex items-center justify-between">
          {/* One readout per width: below `sm` the bar shows the times and
              this row would overflow with them, so it hides them here. */}
          <div className="hidden min-w-25 items-center gap-2 sm:flex">
            <span className="text-sm text-muted-foreground tabular-nums">
              {formatTimecode(currentTime, {
                forceHours: displayedDuration >= 3600,
              })}
            </span>
            <span className="text-muted-foreground">/</span>
            <span className="text-sm text-muted-foreground tabular-nums">
              {formatTimecode(displayedDuration)}
            </span>
          </div>

          <div
            className="flex items-center gap-2"
            role="group"
            aria-label="Playback controls"
          >
            <button
              type="button"
              onClick={onSeekBackward}
              className={cn(
                PLAYER_ICON_BUTTON_CLASS,
                "size-10 hover:bg-accent",
              )}
              aria-label={withShortcut(
                `Seek backward ${MOVIE_SEEK_STEP_SEC} seconds`,
                "J or Left Arrow",
              )}
            >
              <Rewind className="size-5" aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={onTogglePlay}
              className={cn(
                PLAYER_PRIMARY_BUTTON_CLASS,
                "size-14 shadow-lg shadow-primary/20",
              )}
              aria-label={withShortcut(playing ? "Pause" : "Play", "Space or K")}
            >
              {playing ? (
                <Pause className="size-6 fill-current" aria-hidden="true" />
              ) : (
                <Play className="size-6 fill-current" aria-hidden="true" />
              )}
            </button>
            <button
              type="button"
              onClick={onSeekForward}
              className={cn(
                PLAYER_ICON_BUTTON_CLASS,
                "size-10 hover:bg-accent",
              )}
              aria-label={withShortcut(
                `Seek forward ${MOVIE_SEEK_STEP_SEC} seconds`,
                "L or Right Arrow",
              )}
            >
              <FastForward className="size-5" aria-hidden="true" />
            </button>
          </div>

          <div className="flex min-w-25 items-center justify-end gap-2">
            {/* Decorative at phone widths, where it wrapped onto the seek
                button; Playback Settings still exposes the mode there. */}
            <span className="hidden rounded-sm bg-muted/80 px-2 py-1 text-xs text-muted-foreground sm:inline">
              <span className="sr-only">Current playback mode: </span>
              {modeLabel}
            </span>
            <ChapterMenu
              chapters={chapters}
              currentTimeSec={currentTime}
              onSelectChapter={onSelectChapter}
            />
            <VolumeControl
              mediaRef={videoRef}
              variant="minimized"
            />
            <button
              type="button"
              onClick={onToggleFullscreen}
              className={cn(
                PLAYER_ICON_BUTTON_CLASS,
                "size-10 hover:bg-accent",
              )}
              aria-label={withShortcut(fullscreenLabel, "F")}
              aria-pressed={chromeFullscreenMode}
            >
              {chromeFullscreenMode ? (
                <Minimize className="size-5" aria-hidden="true" />
              ) : (
                <Maximize className="size-5" aria-hidden="true" />
              )}
            </button>
          </div>
        </div>
      </div>
    </footer>
  );
}
