import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MOTION_MEDIA_OVERLAY_ENTER_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { UpNextItem } from "@/types/playback";

type Props = {
  item: UpNextItem;
  countdownSec: number;
  /** Fires when the countdown reaches zero or the viewer plays now. */
  onPlay: () => void;
  onCancel: () => void;
};

/**
 * The card shown over a finished video: what plays next, a countdown, and a
 * way out. Mounting starts the countdown; unmounting clears it, so the parent
 * decides when the offer stands by rendering or not rendering the card.
 */
export default function UpNextOverlay({
  item,
  countdownSec,
  onPlay,
  onCancel,
}: Props) {
  const [remainingSec, setRemainingSec] = useState(countdownSec);
  const playButtonRef = useRef<HTMLButtonElement | null>(null);
  // The parent recreates onPlay on every render; without the latch a render
  // after the countdown hit zero would start the hand-off a second time.
  const countdownFiredRef = useRef(false);
  const playLabel = item.resume ? "Resume now" : "Play now";
  const countdownVerb = item.resume ? "Resuming" : "Playing";

  useEffect(() => {
    playButtonRef.current?.focus({ preventScroll: true });
  }, []);

  useEffect(() => {
    const interval = window.setInterval(() => {
      setRemainingSec((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    if (remainingSec > 0 || countdownFiredRef.current) return;
    countdownFiredRef.current = true;
    onPlay();
  }, [remainingSec, onPlay]);

  return (
    // The card sits on the player's click-to-toggle surface (fullscreen only),
    // where a click on the still or the title would otherwise restart the
    // episode that just ended and take the card down with it. The two buttons
    // are the card's only actions.
    // react-doctor-disable-next-line react-doctor/click-events-have-key-events, react-doctor/no-static-element-interactions
    <section
      aria-label="Up next"
      onClick={(event) => event.stopPropagation()}
      className={cn(
        MOTION_MEDIA_OVERLAY_ENTER_CLASS,
        "absolute inset-x-0 bottom-0 z-10 flex justify-center p-4 sm:justify-end sm:p-6",
      )}
    >
      <LiveAnnouncer
        message={`Up next: ${item.title}. ${countdownVerb} in ${countdownSec} seconds.`}
        politeness="polite"
      />
      <div className="flex w-full max-w-sm flex-col gap-3 rounded-lg border border-border bg-background/95 p-4 shadow-2xl shadow-black/40 backdrop-blur-lg">
        <div className="flex items-start gap-3">
          {item.stillUrl ? (
            <img
              src={item.stillUrl}
              alt=""
              className="aspect-video w-28 shrink-0 rounded-md object-cover"
            />
          ) : null}
          <div className="min-w-0">
            <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
              Up next
            </p>
            <p className="truncate text-sm font-semibold text-foreground">
              {item.title}
            </p>
            <p className="text-sm text-muted-foreground tabular-nums">
              {countdownVerb} in {remainingSec}s
            </p>
          </div>
        </div>
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onCancel}
            className="border-border bg-muted text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            Cancel
          </Button>
          <Button ref={playButtonRef} type="button" size="sm" onClick={onPlay}>
            {playLabel}
          </Button>
        </div>
      </div>
    </section>
  );
}
