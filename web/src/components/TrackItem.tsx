import { formatTrackDuration } from "@/lib/format";
import TrackActionsMenu from "@/components/TrackActionsMenu";

export type TrackItemVariant = "album" | "musician" | "library";

type TrackItemProps = {
  // Core track data
  id: number;
  title: string;
  duration: number;

  // Optional display data
  trackIndex?: number;
  subtitle?: string;
  genres?: string[];

  // Navigation data (for actions menu)
  albumId?: number | null;
  albumTitle?: string;
  musicianId?: number | null;
  musicianName?: string;

  // Variant and state
  variant: TrackItemVariant;
  isPlaying?: boolean;
  isCurrentTrack?: boolean;

  // Actions
  onPlay: () => void;
  showActionsMenu?: boolean;
};

export default function TrackItem({
  title,
  duration,
  trackIndex,
  subtitle,
  genres,
  albumId,
  albumTitle,
  musicianId,
  musicianName,
  variant,
  isPlaying = false,
  isCurrentTrack = false,
  onPlay,
  showActionsMenu,
}: TrackItemProps) {
  // Determine if actions menu should show based on variant or explicit prop
  const shouldShowActions = showActionsMenu ?? variant === "library";

  // Play button visibility classes based on variant
  const getPlayButtonClasses = () => {
    const baseClasses =
      "flex size-9 shrink-0 items-center justify-center rounded-full bg-amber-500 text-slate-900 transition-all hover:bg-amber-400";

    if (variant === "library") {
      // Library: always visible
      return baseClasses;
    }

    if (variant === "musician") {
      // Musician: always visible on mobile, hover on desktop
      return `${baseClasses} ${
        isCurrentTrack
          ? "opacity-100"
          : "opacity-100 sm:opacity-0 sm:group-hover:opacity-100"
      }`;
    }

    // Album: hover reveal only
    return `${baseClasses} ${
      isCurrentTrack ? "opacity-100" : "opacity-0 group-hover:opacity-100"
    }`;
  };

  return (
    <div
      className={`group flex animate-in items-center gap-3 px-3 py-3 duration-300 fade-in transition-colors hover:bg-slate-800/50 sm:gap-4 sm:px-4 ${
        isCurrentTrack ? "bg-slate-800/40" : ""
      }`}
    >
      {/* Track index - only for album variant */}
      {variant === "album" && trackIndex != null && (
        <span className="w-8 shrink-0 text-center font-mono text-sm">
          {isPlaying ? (
            <i
              className="fa-solid fa-volume-high animate-pulse text-amber-400"
              aria-hidden="true"
            />
          ) : (
            <span
              className={`${isCurrentTrack ? "text-amber-400" : "text-slate-500"} group-hover:text-amber-400`}
            >
              {trackIndex}
            </span>
          )}
        </span>
      )}

      {/* Track info */}
      <div className="min-w-0 flex-1">
        <p
          className={`truncate font-medium ${isCurrentTrack ? "text-amber-400" : "text-white"}`}
        >
          {title}
        </p>

        {/* Subtitle row - genres for album, text for others */}
        {variant === "album" && genres && genres.length > 0 ? (
          <p className="mt-0.5 truncate text-sm text-amber-400/60">
            {genres.join(", ")}
          </p>
        ) : subtitle ? (
          <p className="truncate text-sm text-slate-400">{subtitle}</p>
        ) : null}
      </div>

      {/* Duration */}
      <span className="shrink-0 text-sm text-slate-500 tabular-nums">
        {formatTrackDuration(duration)}
      </span>

      {/* Actions menu */}
      {shouldShowActions && (
        <TrackActionsMenu
          albumId={albumId}
          albumTitle={albumTitle}
          musicianId={musicianId}
          musicianName={musicianName}
        />
      )}

      {/* Play button - always on right */}
      <button
        onClick={onPlay}
        className={getPlayButtonClasses()}
        title={isPlaying ? "Pause track" : "Play track"}
        aria-label={isPlaying ? `Pause ${title}` : `Play ${title}`}
      >
        {isPlaying ? (
          <i className="fa-solid fa-pause text-xs" aria-hidden="true" />
        ) : (
          <i className="fa-solid fa-play ml-0.5 text-xs" aria-hidden="true" />
        )}
      </button>
    </div>
  );
}
