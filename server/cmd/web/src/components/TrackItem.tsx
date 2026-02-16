import { useState, useEffect, forwardRef } from "react";
import { Volume2, Heart, Pause, Play, GripVertical } from "lucide-react";
import { formatTrackDuration } from "@/lib/format";
import { toggleLikeTrack } from "@/lib/api";
import TrackActionsMenu from "@/components/TrackActionsMenu";

export type TrackItemVariant = "album" | "musician" | "library" | "playlist";

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
  isLiked?: boolean;

  // Actions
  onPlay: () => void;
  showActionsMenu?: boolean;
  onLikeToggle?: (trackId: number, isLiked: boolean) => void;

  // Playlist-specific props
  playlistId?: number;
  canRemoveFromPlaylist?: boolean;
  onRemoveFromPlaylist?: () => void;

  // Drag and drop props
  isDraggable?: boolean;
  isDragging?: boolean;
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement>;
};

const TrackItem = forwardRef<HTMLDivElement, TrackItemProps>(function TrackItem({
  id,
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
  isLiked = false,
  onPlay,
  showActionsMenu,
  onLikeToggle,
  canRemoveFromPlaylist = false,
  onRemoveFromPlaylist,
  isDraggable = false,
  isDragging = false,
  dragHandleProps,
}, ref) {
  const [liked, setLiked] = useState(isLiked);
  const [isLikeLoading, setIsLikeLoading] = useState(false);

  // Sync liked state with prop when it changes externally
  useEffect(() => {
    setLiked(isLiked);
  }, [isLiked]);

  // Determine if actions menu should show based on variant or explicit prop
  const shouldShowActions = showActionsMenu ?? variant === "library";

  const handleLikeClick = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isLikeLoading) return;

    setIsLikeLoading(true);
    try {
      const response = await toggleLikeTrack(id);
      if (!response.error && response.data) {
        setLiked(response.data.is_liked);
        onLikeToggle?.(id, response.data.is_liked);
      }
    } catch (error) {
      console.error("Failed to toggle like:", error);
    } finally {
      setIsLikeLoading(false);
    }
  };

  // Play button visibility classes based on variant
  const getPlayButtonClasses = () => {
    const baseClasses =
      "flex size-9 shrink-0 items-center justify-center rounded-full bg-amber-500 text-slate-900 transition-all hover:bg-amber-400";

    if (variant === "library" || variant === "playlist") {
      // Library and Playlist: always visible
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
      ref={ref}
      className={`group flex items-center gap-3 px-3 py-3 transition-all duration-150 hover:bg-slate-800/50 sm:gap-4 sm:px-4 ${
        isCurrentTrack ? "bg-slate-800/40" : ""
      } ${isDragging ? "opacity-50 shadow-lg ring-2 ring-amber-400/50" : ""}`}
    >
      {/* Drag handle - only for draggable items */}
      {isDraggable && (
        <button
          {...dragHandleProps}
          className="flex size-8 shrink-0 cursor-grab items-center justify-center rounded-sm text-slate-400 transition-colors hover:bg-slate-700 hover:text-slate-300 active:cursor-grabbing"
          aria-label="Drag to reorder"
        >
          <GripVertical className="size-4" aria-hidden="true" />
        </button>
      )}

      {/* Track index - only for album variant */}
      {variant === "album" && trackIndex != null && (
        <span className="w-8 shrink-0 text-center font-mono text-sm">
          {isPlaying ? (
            <Volume2 className="mx-auto size-4 animate-pulse text-amber-400" aria-hidden="true" />
          ) : (
            <span
              className={`${isCurrentTrack ? "text-amber-400" : "text-slate-400"} group-hover:text-amber-400`}
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
      <span className="shrink-0 text-sm text-slate-400 tabular-nums">
        {formatTrackDuration(duration)}
      </span>

      {/* Like button */}
      <button
        onClick={handleLikeClick}
        disabled={isLikeLoading}
        className={`flex size-8 shrink-0 items-center justify-center rounded-full transition-all ${
          liked
            ? "text-red-500 hover:text-red-400"
            : "text-slate-400 hover:text-red-400"
        } ${isLikeLoading ? "opacity-50" : ""}`}
        title={liked ? "Remove from liked" : "Add to liked"}
        aria-label={liked ? `Remove ${title} from liked` : `Add ${title} to liked`}
      >
        <Heart
          className={`size-4 ${liked ? "fill-current" : ""}`}
          aria-hidden="true"
        />
      </button>

      {/* Actions menu */}
      {shouldShowActions && (
        <TrackActionsMenu
          trackId={id}
          trackTitle={title}
          albumId={albumId}
          albumTitle={albumTitle}
          musicianId={musicianId}
          musicianName={musicianName}
          canRemoveFromPlaylist={canRemoveFromPlaylist}
          onRemoveFromPlaylist={onRemoveFromPlaylist}
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
          <Pause className="size-3 fill-current" aria-hidden="true" />
        ) : (
          <Play className="size-3 fill-current" aria-hidden="true" />
        )}
      </button>
    </div>
  );
});

export default TrackItem;
