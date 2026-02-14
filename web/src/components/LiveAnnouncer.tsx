import { useEffect, useState, useRef } from "react";

/**
 * LiveAnnouncer component for screen reader announcements.
 *
 * Uses ARIA live regions to announce dynamic content changes to assistive technologies.
 * The component uses a "double buffer" technique to ensure announcements are always read,
 * even when consecutive announcements contain the same text.
 *
 * @example
 * // Announce when content finishes loading
 * <LiveAnnouncer message={isLoaded ? "12 albums loaded" : ""} />
 *
 * // Announce action results
 * <LiveAnnouncer message={actionMessage} politeness="assertive" />
 */
type LiveAnnouncerProps = {
  /** The message to announce. Empty string or undefined means no announcement. */
  message: string | undefined;
  /**
   * The politeness level of the announcement.
   * - "polite": Wait for current speech to finish (default, use for most cases)
   * - "assertive": Interrupt current speech (use for critical updates only)
   */
  politeness?: "polite" | "assertive";
  /**
   * Delay in milliseconds before making the announcement.
   * Useful to batch rapid updates or wait for animations.
   */
  delay?: number;
};

export default function LiveAnnouncer({
  message,
  politeness = "polite",
  delay = 100,
}: LiveAnnouncerProps) {
  // Use two alternating slots to ensure consecutive identical messages are announced
  const [announcement, setAnnouncement] = useState({ text: "", slot: 0 });
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    // Clear any pending announcement
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    if (!message) {
      return;
    }

    // Delay the announcement slightly to ensure DOM is ready
    timeoutRef.current = setTimeout(() => {
      setAnnouncement((prev) => ({
        text: message,
        slot: prev.slot === 0 ? 1 : 0,
      }));
    }, delay);

    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, [message, delay]);

  // Render two live regions, alternating between them
  // This ensures screen readers always announce even if the text is the same
  return (
    <div className="sr-only" aria-atomic="true">
      <div
        role="status"
        aria-live={politeness}
        aria-relevant="additions text"
      >
        {announcement.slot === 0 ? announcement.text : ""}
      </div>
      <div
        role="status"
        aria-live={politeness}
        aria-relevant="additions text"
      >
        {announcement.slot === 1 ? announcement.text : ""}
      </div>
    </div>
  );
}
