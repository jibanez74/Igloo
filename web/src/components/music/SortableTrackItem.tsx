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

  // dnd-kit hands over the transform and transition as strings for the
  // style attribute, recomputed on every drag frame.
  /* eslint-disable shadcn/no-inline-styles */
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    zIndex: isDragging ? 50 : undefined,
  };
  /* eslint-enable shadcn/no-inline-styles */

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
