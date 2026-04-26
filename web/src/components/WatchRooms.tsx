import { useQuery } from "@tanstack/react-query";
import { watchRoomsQueryOpts } from "@/lib/query-opts";
import { AlertCircle, Radio } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import { Spinner } from "@/components/ui/spinner";
import WatchRoomCard from "@/components/WatchRoomCard";

export default function WatchRooms() {
  const { data, isPending } = useQuery(watchRoomsQueryOpts());

  const rooms = data && !data.error ? (data.data?.rooms ?? []) : [];
  const hasError = data && data.error;
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
      className="mt-6 md:mt-8"
    >
      <LiveAnnouncer message={announcementMessage} />

      <div className="mb-4 rounded-2xl border border-amber-500/20 bg-linear-to-br from-amber-500/10 via-slate-900 to-slate-950 p-5 shadow-[0_20px_60px_-40px_rgba(245,158,11,0.55)]">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-xs font-semibold tracking-[0.2em] text-amber-300 uppercase">
              <Radio className="size-3.5" aria-hidden="true" />
              Shared Watch
            </p>
            <h2
              id="watch-rooms-heading"
              className="mt-2 text-xl font-semibold tracking-tight text-white md:text-2xl"
            >
              Watch Rooms
            </h2>
            <p className="mt-2 max-w-2xl text-sm text-slate-300">
              Jump back into rooms you own or rooms other people invited you to.
            </p>
          </div>

          {!isPending && !hasError && rooms.length > 0 && (
            <p className="rounded-full border border-slate-700 bg-slate-950/60 px-3 py-1 text-xs font-medium text-slate-300">
              {rooms.length} room{rooms.length === 1 ? "" : "s"}
            </p>
          )}
        </div>
      </div>

      {isPending ? (
        <div
          className="flex min-h-24 items-center justify-center"
          role="status"
          aria-label="Loading watch rooms..."
        >
          <Spinner className="size-6 text-amber-400" />
          <span className="sr-only">Loading watch rooms...</span>
        </div>
      ) : hasError ? (
        <Alert
          variant="destructive"
          className="border-red-500/20 bg-red-500/10 text-red-400"
        >
          <AlertCircle className="size-4" aria-hidden="true" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>
            {data.message || "Failed to load watch rooms. Please try again later."}
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <span
            tabIndex={0}
            className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-amber-400 focus:px-4 focus:py-2 focus:text-slate-900"
            aria-label={`Watch Rooms section, ${rooms.length} room${rooms.length === 1 ? "" : "s"} available`}
          >
            Watch Rooms - {rooms.length} room{rooms.length === 1 ? "" : "s"}
          </span>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {rooms.map((room) => (
            <WatchRoomCard key={room.id} room={room} />
          ))}
          </div>
        </>
      )}
    </section>
  );
}
