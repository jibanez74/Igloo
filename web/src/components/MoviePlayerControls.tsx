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
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";
import { formatTimeSeconds } from "@/lib/format";
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
              "absolute inset-x-0 bottom-0 z-10 border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg",
              controlsVisible
                ? "translate-y-0 opacity-100"
                : "pointer-events-none translate-y-full opacity-0",
            )
          : "shrink-0 border-t border-slate-700/50 bg-slate-900/95 p-4 backdrop-blur-lg"
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
            <span className="text-sm text-slate-400 tabular-nums">
              {formatTimeSeconds(currentTime)}
            </span>
            <span className="text-slate-600">/</span>
            <span className="text-sm text-slate-400 tabular-nums">
              {formatTimeSeconds(displayedDuration)}
            </span>
          </div>

          <div
            className="flex items-center gap-2"
            role="group"
            aria-label="Playback controls"
          >
            <button
              onClick={onSeekBackward}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-slate-300 hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none",
              )}
              aria-label="Seek backward 10 seconds"
            >
              <Rewind className="size-5" aria-hidden="true" />
            </button>
            <button
              onClick={onTogglePlay}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-14 items-center justify-center rounded-full bg-cyan-500 text-slate-900 shadow-lg shadow-cyan-500/20 hover:bg-cyan-400 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none",
              )}
              aria-label={playing ? "Pause" : "Play"}
            >
              {playing ? (
                <Pause className="size-6 fill-current" aria-hidden="true" />
              ) : (
                <Play className="size-6 fill-current" aria-hidden="true" />
              )}
            </button>
            <button
              onClick={onSeekForward}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-slate-300 hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none",
              )}
              aria-label="Seek forward 10 seconds"
            >
              <FastForward className="size-5" aria-hidden="true" />
            </button>
          </div>

          <div className="flex min-w-25 items-center justify-end gap-2">
            <span
              className="rounded-sm bg-slate-800/80 px-2 py-1 text-xs text-slate-400"
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
              accent="cyan"
            />
            <button
              onClick={onToggleFullscreen}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-10 items-center justify-center rounded-full text-slate-300 hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none",
              )}
              aria-label={
                chromeFullscreenMode
                  ? isImmersiveViewport && !isFullscreen
                    ? "Exit expanded view"
                    : "Exit fullscreen"
                  : "Fullscreen"
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
