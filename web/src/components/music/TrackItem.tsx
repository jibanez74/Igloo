import { Disc3, Volume2, Heart, Pause, Play, GripVertical } from "lucide-react";
import { formatTrackDuration } from "@/lib/format";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import { useLikeButtonState } from "@/hooks/useTrackLikeToggle";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_TRACK_ICON_BUTTON_CLASS,
  MOTION_TRACK_MENU_TRIGGER_CLASS,
  MOTION_TRACK_PLAY_BUTTON_CLASS,
  MOTION_TRACK_ROW_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import TrackActionsMenu from "@/components/music/TrackActionsMenu";
import type { TrackItemVariant } from "@/types";

export type TrackItemProps = {
  // Core track data
  id: number;
  title: string;
  duration: number;

  // Optional display data
  trackIndex?: number;
  // Spotify's explicit-lyrics flag for a paired track.
  explicit?: boolean;
  subtitle?: string;
  genres?: string[];
  // The album cover for the row's thumbnail. Leave it out for no thumbnail
  // (the album page, where every row would repeat the cover above the list);
  // null draws the placeholder tile, so a list's titles stay aligned.
  coverUrl?: string | null;

  // Navigation data (for actions menu)
  albumId?: number | null;
  musicianId?: number | null;

  // Variant and state
  variant: TrackItemVariant;
  isPlaying?: boolean;
  isCurrentTrack?: boolean;

  // Actions
  onPlay: () => void;
  showActionsMenu?: boolean;

  // Playlist-specific props
  canRemoveFromPlaylist?: boolean;
  onRemoveFromPlaylist?: () => void;

  // Drag and drop props
  isDraggable?: boolean;
  isDragging?: boolean;
  dragHandleProps?: React.HTMLAttributes<HTMLButtonElement>;
};

// A 40 px cover at the start of a row. It repeats what the row's text already
// says, so it is hidden from assistive tech. 40 px sits inside the two text
// lines' height, so the row keeps VIRTUAL_LIST_TRACK_HEIGHT. Below `sm` the
// row gives it up: the title needs that room beside the row's controls.
function TrackCover({ coverUrl }: { coverUrl: string | null }) {
  const { showPoster, onError } = usePosterFallback(coverUrl ?? "");

  return (
    <span
      data-slot="track-cover"
      aria-hidden="true"
      className="hidden size-10 shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted sm:flex"
    >
      {showPoster ? (
        <img
          src={coverUrl ?? ""}
          alt=""
          width={40}
          height={40}
          loading="lazy"
          decoding="async"
          fetchPriority="low"
          className="size-full object-cover"
          onError={onError}
        />
      ) : (
        <Disc3 className="size-4 text-muted-foreground" />
      )}
    </span>
  );
}

// The flag count is arity, not complexity: each one drives a single independent
// thing (a class, an icon, one rendered block) and none is read together with
// another, so the nominal combinations never arise. Every call site passes a
// fixed preset determined by `variant`.
// react-doctor-disable-next-line react-doctor/no-many-boolean-props
export default function TrackItem({
  id,
  title,
  duration,
  trackIndex,
  explicit = false,
  subtitle,
  genres,
  coverUrl,
  albumId,
  musicianId,
  variant,
  isPlaying = false,
  isCurrentTrack = false,
  onPlay,
  showActionsMenu,
  canRemoveFromPlaylist = false,
  onRemoveFromPlaylist,
  isDraggable = false,
  isDragging = false,
  dragHandleProps,
}: TrackItemProps) {
  const likeButton = useLikeButtonState(id, title);
  const durationLabel = formatTrackDuration(duration);
  const secondLine =
    variant === "album"
      ? genres && genres.length > 0
        ? genres.join(", ")
        : null
      : (subtitle ?? null);

  // Every row offers its actions menu (Add to Playlist at least) unless the
  // caller opts out; the album and musician pages pass only the link that
  // leads somewhere else, so a row never links back to the page it is on.
  const shouldShowActions = showActionsMenu ?? true;

  const handleLikeClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    likeButton.toggle();
  };

  // The play button is always there, but a list of filled buttons shouts, so
  // it is quiet (an unfilled, muted icon) until the row is hovered or the
  // button focused. The current track keeps the filled button, and so does
  // every row where nothing hovers (`pointer-coarse:`).
  const playButtonClassName = cn(
    "flex size-9 shrink-0 items-center justify-center rounded-full hover:bg-primary/90",
    FOCUS_VISIBLE_RING_CLASS,
    MOTION_TRACK_PLAY_BUTTON_CLASS,
    isCurrentTrack
      ? "bg-primary text-primary-foreground"
      : "text-muted-foreground group-hover:bg-primary group-hover:text-primary-foreground focus-visible:bg-primary focus-visible:text-primary-foreground pointer-coarse:bg-primary pointer-coarse:text-primary-foreground",
  );

  return (
    <div
      className={cn(
        "group flex items-center gap-2 p-3 hover:bg-muted/50 sm:gap-4 sm:px-4",
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

      {coverUrl !== undefined && <TrackCover coverUrl={coverUrl} />}

      {/* Track info */}
      <div className="min-w-0 flex-1">
        <p
          className={cn(
            "flex items-center gap-2 font-medium",
            isCurrentTrack ? "text-primary" : "text-foreground",
          )}
        >
          <span className="truncate">{title}</span>
          {explicit && (
            <Badge
              variant="outline"
              className="shrink-0 px-1.5 py-0 text-[10px]/4 font-semibold"
            >
              <span aria-hidden="true">E</span>
              <span className="sr-only">Explicit</span>
            </Badge>
          )}
        </p>

        {/* Second line: genres for album, subtitle for others; below `sm` the
            duration joins it (or stands alone) so the title keeps its room
            beside the row's controls. Exactly one duration is in the
            accessibility tree at any width: this one is `sm:hidden`, the
            right-hand one `hidden sm:inline`. */}
        {(secondLine !== null || durationLabel) && (
          <p
            className={cn(
              "flex items-center gap-1 text-sm text-muted-foreground",
              variant === "album" && "mt-0.5",
              secondLine === null && "sm:hidden",
            )}
          >
            {secondLine !== null && (
              <span className="truncate">{secondLine}</span>
            )}
            <span className="flex shrink-0 items-center gap-1 sm:hidden">
              {secondLine !== null && <span aria-hidden="true">·</span>}
              <span className="tabular-nums">{durationLabel}</span>
            </span>
          </p>
        )}
      </div>

      {/* Duration */}
      <span className="hidden shrink-0 text-sm text-muted-foreground tabular-nums sm:inline">
        {durationLabel}
      </span>

      {/* Like button */}
      <button
        type="button"
        onClick={handleLikeClick}
        aria-disabled={likeButton.isDisabled || undefined}
        className={cn(
          "flex size-8 shrink-0 items-center justify-center rounded-full",
          FOCUS_VISIBLE_RING_CLASS,
          MOTION_TRACK_ICON_BUTTON_CLASS,
          likeButton.isLiked
            ? "text-destructive"
            : "text-muted-foreground hover:text-destructive",
          likeButton.isDisabled && "cursor-not-allowed opacity-50",
        )}
        aria-label={likeButton.ariaLabel}
        aria-pressed={likeButton.ariaPressed}
        aria-busy={likeButton.ariaBusy}
      >
        <Heart
          className={`size-4 ${likeButton.isLiked ? "fill-current" : ""}`}
          aria-hidden="true"
        />
      </button>

      {/* Actions menu */}
      {shouldShowActions && (
        <TrackActionsMenu
          trackId={id}
          trackTitle={title}
          albumId={albumId}
          musicianId={musicianId}
          canRemoveFromPlaylist={canRemoveFromPlaylist}
          onRemoveFromPlaylist={onRemoveFromPlaylist}
        />
      )}

      {/* Play button - always on right */}
      <button
        type="button"
        onClick={onPlay}
        className={playButtonClassName}
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
