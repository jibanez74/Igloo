import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Volume2, Heart, Pause, Play, GripVertical } from "lucide-react";
import { formatTrackDuration } from "@/lib/format";
import { toggleLikeTrack } from "@/lib/api";
import {
  FOCUS_VISIBLE_RING_CLASS,
  LIKED_TRACK_IDS_KEY,
  LIKED_TRACKS_KEY,
  MOTION_LOADING_STATE_CLASS,
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import TrackActionsMenu from "@/components/TrackActionsMenu";
import type { TrackItemVariant } from "@/types";

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

export default function TrackItem({
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
}: TrackItemProps) {
  const queryClient = useQueryClient();
  const [isLikeLoading, setIsLikeLoading] = useState(false);

  // Determine if actions menu should show based on variant or explicit prop
  const shouldShowActions = showActionsMenu ?? variant === "library";

  const handleLikeClick = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isLikeLoading) return;

    setIsLikeLoading(true);
    try {
      const response = await toggleLikeTrack(id);
      if (!response.error && response.data) {
        onLikeToggle?.(id, response.data.is_liked);
        queryClient.invalidateQueries({ queryKey: [LIKED_TRACKS_KEY] });
        queryClient.invalidateQueries({ queryKey: [LIKED_TRACK_IDS_KEY] });
      }
    } catch (error) {
      console.error("Failed to toggle like:", error);
    }

    setIsLikeLoading(false);
  };

  // Play button visibility classes based on variant
  const getPlayButtonClasses = () => {
    const baseClasses = cn(
      "flex size-9 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground hover:bg-primary/90",
      FOCUS_VISIBLE_RING_CLASS,
      MOTION_TRACK_PLAY_BUTTON_CLASS,
    );

    if (variant === "library" || variant === "playlist") {
      // Library and Playlist: always visible
      return baseClasses;
    }

    // Album / Musician: hover on desktop; always visible on touch / small screens
    return cn(
      baseClasses,
      isCurrentTrack
        ? "opacity-100"
        : "opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100",
    );
  };

  return (
    <div
      className={cn(
        "group flex items-center gap-3 p-3 hover:bg-muted/50 sm:gap-4 sm:px-4",
        MOTION_TRACK_ROW_CLASS,
        isCurrentTrack && "bg-muted/40",
        isDragging && "opacity-50 shadow-lg ring-2 ring-ring/50",
      )}
      aria-current={isCurrentTrack || undefined}
    >
      {/* Drag handle - only for draggable items */}
      {isDraggable && (
        <button
          type="button"
          {...dragHandleProps}
          className={cn(
            "flex size-8 shrink-0 cursor-grab items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-muted-foreground active:cursor-grabbing",
            FOCUS_VISIBLE_RING_CLASS,
            MOTION_TRACK_MENU_TRIGGER_CLASS,
          )}
          aria-label="Drag to reorder"
        >
          <GripVertical className="size-4" aria-hidden="true" />
        </button>
      )}

      {/* Track index - only for album variant */}
      {variant === "album" && trackIndex != null && (
        <span className="w-8 shrink-0 text-center font-mono text-sm">
          {isPlaying ? (
            <Volume2
              className={cn("mx-auto size-4 text-primary", MOTION_LOADING_STATE_CLASS)}
              aria-hidden="true"
            />
          ) : (
            <span
              className={`${isCurrentTrack ? "text-primary" : "text-muted-foreground"} group-hover:text-primary`}
            >
              {trackIndex}
            </span>
          )}
        </span>
      )}

      {/* Track info */}
      <div className="min-w-0 flex-1">
        <p
          className={`truncate font-medium ${isCurrentTrack ? "text-primary" : "text-foreground"}`}
        >
          {title}
        </p>

        {/* Subtitle row - genres for album, text for others */}
        {variant === "album" && genres && genres.length > 0 ? (
          <p className="mt-0.5 truncate text-sm text-primary/60">
            {genres.join(", ")}
          </p>
        ) : subtitle ? (
          <p className="truncate text-sm text-muted-foreground">{subtitle}</p>
        ) : null}
      </div>

      {/* Duration */}
      <span className="shrink-0 text-sm text-muted-foreground tabular-nums">
        {formatTrackDuration(duration)}
      </span>

      {/* Like button */}
      <button
        type="button"
        onClick={handleLikeClick}
        disabled={isLikeLoading}
        className={cn(
          "flex size-8 shrink-0 items-center justify-center rounded-full",
          FOCUS_VISIBLE_RING_CLASS,
          MOTION_TRACK_ICON_BUTTON_CLASS,
          isLiked
            ? "text-destructive"
            : "text-muted-foreground hover:text-destructive",
          isLikeLoading && "opacity-50",
        )}
        aria-label={isLiked ? `Remove ${title} from liked` : `Add ${title} to liked`}
      >
        <Heart
          className={`size-4 ${isLiked ? "fill-current" : ""}`}
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
        type="button"
        onClick={onPlay}
        className={getPlayButtonClasses()}
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
}
