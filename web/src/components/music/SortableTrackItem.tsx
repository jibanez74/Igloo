import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import TrackItem, { type TrackItemProps } from "./TrackItem";

// A dnd-kit adapter over TrackItem: it owns the sortable wiring and forwards
// everything else, so its props are TrackItem's minus the drag state it
// supplies itself.
type SortableTrackItemProps = Omit<
  TrackItemProps,
  "isDraggable" | "isDragging" | "dragHandleProps"
> & {
  /** Must be unique within the list. */
  sortableId: number;
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
