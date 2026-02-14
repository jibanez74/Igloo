import { useState } from "react";
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  TouchSensor,
  useSensor,
  useSensors,
  DragOverlay,
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
import { unwrapString, unwrapInt, unwrapStringOrUndefined } from "@/lib/nullable";
import type { PlaylistTrackType } from "@/types";

type DraggableTrackListProps = {
  tracks: PlaylistTrackType[];
  playlistId: number;
  playlistName: string;
  coverUrl: string | null;
  canEdit: boolean;
  onReorder: (trackIds: number[]) => void;
  onPlayTrack: (track: PlaylistTrackType) => void;
  onRemoveTrack: (trackId: number) => void;
  currentTrackId?: number;
  isPlaying: boolean;
};

export default function DraggableTrackList({
  tracks,
  playlistId,
  canEdit,
  onReorder,
  onPlayTrack,
  onRemoveTrack,
  currentTrackId,
  isPlaying,
}: DraggableTrackListProps) {
  const [activeId, setActiveId] = useState<UniqueIdentifier | null>(null);

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

  // Custom announcements for screen readers
  const announcements = {
    onDragStart({ active }: DragStartEvent) {
      const track = tracks.find((t) => t.id === active.id);
      return `Picked up ${track?.title || "track"}. Press space to drop, or escape to cancel.`;
    },
    onDragOver({ active, over }: { active: { id: UniqueIdentifier }; over: { id: UniqueIdentifier } | null }) {
      if (!over) return;
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
    <div className="overflow-hidden rounded-xl border border-amber-500/10 bg-slate-800/30">
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        onDragCancel={handleDragCancel}
        modifiers={[restrictToVerticalAxis]}
        accessibility={{
          announcements,
        }}
      >
        <SortableContext
          items={sortableIds}
          strategy={verticalListSortingStrategy}
        >
          <div className="divide-y divide-slate-700/30">
            {tracks.map((track) => (
              <SortableTrackItem
                key={track.id}
                sortableId={track.id}
                id={track.id}
                title={track.title}
                duration={track.duration}
                subtitle={unwrapString(track.musician_name) ?? "Unknown Artist"}
                albumId={unwrapInt(track.album_id)}
                albumTitle={unwrapStringOrUndefined(track.album_title)}
                musicianId={unwrapInt(track.musician_id)}
                musicianName={unwrapStringOrUndefined(track.musician_name)}
                variant="playlist"
                isPlaying={currentTrackId === track.id && isPlaying}
                isCurrentTrack={currentTrackId === track.id}
                onPlay={() => onPlayTrack(track)}
                showActionsMenu
                playlistId={playlistId}
                canRemoveFromPlaylist={canEdit}
                onRemoveFromPlaylist={() => onRemoveTrack(track.id)}
              />
            ))}
          </div>
        </SortableContext>

        {/* Drag overlay for better visual feedback */}
        <DragOverlay>
          {activeTrack ? (
            <div className="rounded-lg bg-slate-900 shadow-2xl ring-2 ring-amber-400">
              <TrackItem
                id={activeTrack.id}
                title={activeTrack.title}
                duration={activeTrack.duration}
                subtitle={
                  activeTrack.musician_name?.Valid
                    ? activeTrack.musician_name.String
                    : "Unknown Artist"
                }
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
