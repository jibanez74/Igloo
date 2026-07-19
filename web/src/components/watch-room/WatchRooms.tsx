import { useQuery } from "@tanstack/react-query";
import { watchRoomsQueryOpts } from "@/lib/query-opts";
import { Radio } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { Spinner } from "@/components/ui/spinner";
import SectionErrorAlert from "@/components/shared/SectionErrorAlert";
import WatchRoomCard from "@/components/watch-room/WatchRoomCard";
import { MOTION_SECTION_ENTER_DELAYED_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

const WATCH_ROOMS_DESCRIPTION_ID = "watch-rooms-description";
const WATCH_ROOMS_SUMMARY_ID = "watch-rooms-summary";

export default function WatchRooms() {
  const { data, isPending } = useQuery(watchRoomsQueryOpts());

  const rooms = data && !data.error ? (data.data?.rooms ?? []) : [];
  const hasError = data && data.error;
  const sectionDescribedBy =
    !isPending && !hasError && rooms.length > 0
      ? `${WATCH_ROOMS_DESCRIPTION_ID} ${WATCH_ROOMS_SUMMARY_ID}`
      : WATCH_ROOMS_DESCRIPTION_ID;
  const announcementMessage = isPending
    ? undefined
    : hasError
      ? data.message || "Failed to load watch rooms"
      : rooms.length === 0
        ? undefined
        : `${rooms.length} watch room${rooms.length === 1 ? "" : "s"} available`;

  // Render nothing when there are no rooms and no error — keeps the home page clean
  if (!isPending && !hasError && rooms.length === 0) {
    return null;
  }

  return (
    <section
      role="region"
      aria-labelledby="watch-rooms-heading"
      aria-describedby={sectionDescribedBy}
      className={cn("mt-6 md:mt-8", MOTION_SECTION_ENTER_DELAYED_CLASS)}
    >
      <LiveAnnouncer message={announcementMessage} />

      <div className="mb-4 rounded-2xl border border-primary/20 bg-linear-to-br from-primary/10 via-card to-background p-5 shadow-[0_20px_60px_-40px] shadow-primary/55">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-xs font-semibold tracking-[0.2em] text-primary uppercase">
              <Radio className="size-3.5" aria-hidden="true" />
              Shared Watch
            </p>
            <h2
              id="watch-rooms-heading"
              className="mt-2 text-xl font-semibold tracking-tight text-foreground md:text-2xl"
            >
              Watch Rooms
            </h2>
            <p
              id={WATCH_ROOMS_DESCRIPTION_ID}
              className="mt-2 max-w-2xl text-sm text-muted-foreground"
            >
              Jump back into rooms you own or rooms other people invited you to.
            </p>
          </div>

          {!isPending && !hasError && rooms.length > 0 && (
            <Badge
              id={WATCH_ROOMS_SUMMARY_ID}
              variant="outline"
              className="px-3 py-1"
            >
              {rooms.length} room{rooms.length === 1 ? "" : "s"}
            </Badge>
          )}
        </div>
      </div>

      {isPending ? (
        <div
          className="flex min-h-24 items-center justify-center"
          role="status"
          aria-label="Loading watch rooms..."
        >
          <Spinner className="size-6 text-primary" />
        </div>
      ) : hasError ? (
        <SectionErrorAlert
          message={
            data.message || "Failed to load watch rooms. Please try again later."
          }
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {rooms.map((room) => (
            <WatchRoomCard key={room.id} room={room} />
          ))}
        </div>
      )}
    </section>
  );
}
