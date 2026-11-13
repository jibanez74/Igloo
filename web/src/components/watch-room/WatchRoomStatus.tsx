import { AlertCircle, ArrowLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";

// The room's loading and unavailable screens, apart from the room itself:
// the route renders them as its pending view and for a malformed link, and a
// route's pending view loads with the app's entry chunk, so importing them
// from WatchRoomPage would bring the whole player into it.

export function WatchRoomLoading() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center px-4">
      <div className="text-center">
        <Spinner size="2xl" tone="primary" className="mx-auto mb-4" />
        <p className="text-lg font-medium text-foreground">
          Loading watch room...
        </p>
      </div>
    </div>
  );
}

type WatchRoomUnavailableProps = {
  message: string;
  onBackHome: () => void;
};

export function WatchRoomUnavailable({
  message,
  onBackHome,
}: WatchRoomUnavailableProps) {
  return (
    <div className="flex min-h-[60vh] items-center justify-center px-4">
      <div className="max-w-md text-center">
        <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-full bg-destructive/10">
          <AlertCircle
            className="size-10 text-destructive"
            aria-hidden="true"
          />
        </div>
        <h1 className="mb-2 text-xl font-semibold text-foreground">
          Watch room unavailable
        </h1>
        <p className="mb-6 text-muted-foreground">{message}</p>
        <Button
          size="xl"
          shape="pill"
          onClick={onBackHome}
          className="font-semibold shadow-glow"
        >
          <ArrowLeft className="size-5" aria-hidden="true" />
          Back home
        </Button>
      </div>
    </div>
  );
}
