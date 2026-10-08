import { useRef, useState } from "react";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
  DragOverlay,
  defaultScreenReaderInstructions,
} from "@dnd-kit/core";
import type { DragEndEvent, DragStartEvent, UniqueIdentifier } from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  arrayMove,
} from "@dnd-kit/sortable";
import { restrictToVerticalAxis } from "@dnd-kit/modifiers";
import SortableTrackItem from "./SortableTrackItem";
import TrackItem from "./TrackItem";
import { trackRowProps } from "@/lib/track-row-props";
import { useTrackPlaybackMatcher } from "@/hooks/useTrackPlaybackMatcher";
import { useShortcutHints } from "@/hooks/useShortcutHints";
import type { PlaylistTrackType } from "@/types";

type DraggableTrackListProps = {
  tracks: PlaylistTrackType[];
  canEdit: boolean;
  onReorder: (trackIds: number[]) => void;
  onPlayTrack: (track: PlaylistTrackType) => void;
  onRemoveTrack: (trackId: number) => void;
};

export default function DraggableTrackList({
  tracks,
  canEdit,
  onReorder,
  onPlayTrack,
  onRemoveTrack,
}: DraggableTrackListProps) {
  const matchTrackPlayback = useTrackPlaybackMatcher();
  const { showShortcutHints } = useShortcutHints();
  const [activeId, setActiveId] = useState<UniqueIdentifier | null>(null);
  // dnd-kit hands announcements only the active item, so the drag-start
  // handler records whether the keyboard started the drag.
  const keyboardDragRef = useRef(false);

  // Configure sensors with activation constraints
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8, // 8px movement required before drag starts
      },
    }),
    useSensor(TouchSensor, {
      activationConstraint: {
        delay: 200, // 200ms hold before drag starts on touch
        tolerance: 5,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  );

  const handleDragStart = (event: DragStartEvent) => {
    keyboardDragRef.current = event.activatorEvent instanceof KeyboardEvent;
    setActiveId(event.active.id);
  };

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    setActiveId(null);

    if (over && active.id !== over.id) {
      const oldIndex = tracks.findIndex((t) => t.id === active.id);
      const newIndex = tracks.findIndex((t) => t.id === over.id);

      if (oldIndex !== -1 && newIndex !== -1) {
        const newOrder = arrayMove(tracks, oldIndex, newIndex);
        onReorder(newOrder.map((t) => t.id));
      }
    }
  };

  const handleDragCancel = () => {
    setActiveId(null);
  };

  // Find the active track for the drag overlay
  const activeTrack = activeId
    ? tracks.find((t) => t.id === activeId)
    : null;

  // Create sortable IDs from track IDs
  const sortableIds = tracks.map((t) => t.id);

  // The handle's described-by text. A touch-first device has no space bar,
  // so it gets the hold-and-drag gesture instead of dnd-kit's key map.
  const screenReaderInstructions = showShortcutHints
    ? defaultScreenReaderInstructions
    : { draggable: "Touch and hold a track, then drag it to a new position." };

  // Custom announcements for screen readers
  const announcements = {
    onDragStart({ active }: Pick<DragStartEvent, "active">) {
      const track = tracks.find((t) => t.id === active.id);
      const pickedUp = `Picked up ${track?.title || "track"}.`;
      // Only a keyboard drag is dropped with space; a pointer or touch drag
      // drops on release.
      return keyboardDragRef.current
        ? `${pickedUp} Press space to drop, or escape to cancel.`
        : pickedUp;
    },
    onDragOver({ active, over }: { active: { id: UniqueIdentifier }; over: { id: UniqueIdentifier } | null }) {
      // A drag starts over its own slot; announcing that would replace the
      // pick-up message in the live region before it is read.
      if (!over || over.id === active.id) return;
      const activeTrack = tracks.find((t) => t.id === active.id);
      const overTrack = tracks.find((t) => t.id === over.id);
      if (activeTrack && overTrack) {
        return `${activeTrack.title} is over ${overTrack.title}`;
      }
    },
    onDragEnd({ active, over }: DragEndEvent) {
      const activeTrack = tracks.find((t) => t.id === active.id);
      if (over) {
        const overTrack = tracks.find((t) => t.id === over.id);
        if (activeTrack && overTrack && active.id !== over.id) {
          return `${activeTrack.title} was moved after ${overTrack.title}`;
        }
      }
      return `${activeTrack?.title || "Track"} was dropped`;
    },
    onDragCancel({ active }: { active: { id: UniqueIdentifier } }) {
      const track = tracks.find((t) => t.id === active.id);
      return `Dragging cancelled. ${track?.title || "Track"} was returned to its original position.`;
    },
  };

  return (
    <div className="overflow-hidden rounded-xl border border-primary/10 bg-muted/30">
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        onDragCancel={handleDragCancel}
        modifiers={[restrictToVerticalAxis]}
        accessibility={{
          announcements,
          screenReaderInstructions,
        }}
      >
        <SortableContext
          items={sortableIds}
          strategy={verticalListSortingStrategy}
        >
          <div className="divide-y divide-border/30">
            {tracks.map((track) => (
              <SortableTrackItem
                key={track.id}
                sortableId={track.id}
                {...trackRowProps(track)}
                variant="playlist"
                {...matchTrackPlayback(track.id)}
                onPlay={() => onPlayTrack(track)}
                showActionsMenu
                canRemoveFromPlaylist={canEdit}
                onRemoveFromPlaylist={() => onRemoveTrack(track.id)}
              />
            ))}
          </div>
        </SortableContext>

        {/* Drag overlay for better visual feedback */}
        <DragOverlay>
          {activeTrack ? (
            <div className="rounded-lg bg-card shadow-2xl ring-2 ring-ring">
              <TrackItem
                {...trackRowProps(activeTrack)}
                variant="playlist"
                isPlaying={false}
                isCurrentTrack={false}
                onPlay={() => {}}
                isDraggable
                isDragging
              />
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </div>
  );
}
