import type { RefObject } from "react";
import {
  FastForward,
  Maximize,
  Minimize,
  Pause,
  Play,
  Rewind,
} from "lucide-react";
import ProgressBar from "@/components/ProgressBar";
import ChapterMenu from "@/components/ChapterMenu";
import VolumeControl from "@/components/VolumeControl";
import {
  MOVIE_SEEK_STEP_SEC,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";
import { formatTimecode } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ChapterType } from "@/types";

type MoviePlayerControlsProps = {
  chromeFullscreenMode: boolean;
  controlsVisible: boolean;
  isFullscreen: boolean;
  isImmersiveViewport: boolean;
  currentTime: number;
  duration: number;
  displayedDuration: number;
  playing: boolean;
  qualityLabel: string;
  chapters: ChapterType[];
  videoRef: RefObject<HTMLVideoElement | null>;
  onSeek: (time: number) => void;
  onSeekBackward: () => void;
  onSeekForward: () => void;
  onTogglePlay: () => void;
  onToggleFullscreen: () => void;
  onSelectChapter: (startTimeSec: number, title: string) => void;
};

export default function MoviePlayerControls({
  chromeFullscreenMode,
  controlsVisible,
  isFullscreen,
  isImmersiveViewport,
  currentTime,
  duration,
  displayedDuration,
  playing,
  qualityLabel,
  chapters,
  videoRef,
  onSeek,
  onSeekBackward,
  onSeekForward,
  onTogglePlay,
  onToggleFullscreen,
  onSelectChapter,
}: MoviePlayerControlsProps) {
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
            onSeek={onSeek}
          />
        </div>

        <div className="flex items-center justify-between">
          <div className="flex min-w-25 items-center gap-2">
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
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
              )}
              aria-label={`Seek backward ${MOVIE_SEEK_STEP_SEC} seconds (J or Left Arrow)`}
            >
              <Rewind className="size-5" aria-hidden="true" />
            </button>
            <button
              type="button"
              onClick={onTogglePlay}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg shadow-primary/20 hover:bg-primary/90 focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background focus:outline-hidden",
              )}
              aria-label={playing ? "Pause (Space or K)" : "Play (Space or K)"}
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
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
              )}
              aria-label={`Seek forward ${MOVIE_SEEK_STEP_SEC} seconds (L or Right Arrow)`}
            >
              <FastForward className="size-5" aria-hidden="true" />
            </button>
          </div>

          <div className="flex min-w-25 items-center justify-end gap-2">
            <span
              className="rounded-sm bg-muted/80 px-2 py-1 text-xs text-muted-foreground"
              aria-label="Current stream quality"
            >
              {qualityLabel}
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
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:ring-ring focus:outline-hidden",
              )}
              aria-label={
                chromeFullscreenMode
                  ? isImmersiveViewport && !isFullscreen
                    ? "Exit expanded view (F)"
                    : "Exit fullscreen (F)"
                  : "Fullscreen (F)"
              }
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
