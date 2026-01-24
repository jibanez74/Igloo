import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import TrackItem, { type TrackItemVariant } from "./TrackItem";

type SortableTrackItemProps = {
  // Sortable ID (must be unique)
  sortableId: number;

  // Core track data
  id: number;
  title: string;
  duration: number;

  // Optional display data
  trackIndex?: number;
  subtitle?: string;
  genres?: string[];

  // Navigation data
  albumId?: number | null;
  albumTitle?: string;
  musicianId?: number | null;
  musicianName?: string;

  // State
  variant: TrackItemVariant;
  isPlaying?: boolean;
  isCurrentTrack?: boolean;
  isLiked?: boolean;

  // Actions
  onPlay: () => void;
  showActionsMenu?: boolean;
  onLikeToggle?: (trackId: number, isLiked: boolean) => void;

  // Playlist-specific
  playlistId?: number;
  canRemoveFromPlaylist?: boolean;
  onRemoveFromPlaylist?: () => void;
};

export default function SortableTrackItem({
  sortableId,
  ...trackProps
}: SortableTrackItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: sortableId,
    data: {
      title: trackProps.title,
    },
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 50 : undefined,
  };

  return (
    <div ref={setNodeRef} style={style}>
      <TrackItem
        {...trackProps}
        isDraggable
        isDragging={isDragging}
        dragHandleProps={{
          ...attributes,
          ...listeners,
        }}
      />
    </div>
  );
}
