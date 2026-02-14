import { useEffect, useRef } from "react";
import { Snowflake, Hammer, type LucideIcon } from "lucide-react";

type ComingSoonProps = {
  /** Page/feature title */
  title: string;
  /** Description of the upcoming feature */
  description?: string;
  /** Lucide icon component */
  icon?: LucideIcon;
};

/**
 * A full-screen placeholder component for pages/features still in development.
 * Includes entrance animations and matches the Igloo theme.
 */
export default function ComingSoon({
  title,
  description = "We're working hard to bring you this feature. Check back soon for updates!",
  icon: Icon = Snowflake,
}: ComingSoonProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  // Entrance animation
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    // Check for reduced motion preference
    const prefersReduced = window.matchMedia(
      "(prefers-reduced-motion: reduce)"
    ).matches;

    if (prefersReduced) {
      container.classList.remove("opacity-0", "translate-y-4");
      return;
    }

    // Trigger animation on next frame
    requestAnimationFrame(() => {
      container.classList.remove("opacity-0", "translate-y-4");
    });
  }, []);

  // Accessible announcement for screen readers
  const announcement = `${title}. Under Development. ${description}`;

  return (
    <section
      aria-labelledby="coming-soon-title"
      aria-describedby="coming-soon-desc"
      className="flex min-h-[60vh] flex-col items-center justify-center px-4 py-16 text-center"
    >
      {/* Screen reader announcement - focusable for Tab navigation */}
      <span
        tabIndex={0}
        className="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50
          focus:rounded-md focus:bg-slate-800 focus:px-4 focus:py-2 focus:text-white focus:ring-2
          focus:ring-amber-400 focus:outline-none"
        aria-label={announcement}
      >
        {title} - Under Development
      </span>

      <div
        ref={containerRef}
        className="translate-y-4 opacity-0 transition-all duration-700 ease-out will-change-transform"
      >
        {/* Animated icon container */}
        <div className="relative mx-auto mb-8" aria-hidden="true">
          {/* Outer glow ring */}
          <div className="absolute inset-0 animate-ping rounded-full bg-amber-400/20" />

          {/* Icon circle */}
          <div
            className="relative flex size-24 items-center justify-center rounded-full
              bg-linear-to-br from-slate-700 to-slate-800 shadow-xl
              ring-4 ring-slate-700/50 sm:size-28 md:size-32"
          >
            <Icon
              className="size-10 text-amber-400 sm:size-12 md:size-14"
              aria-hidden="true"
            />
          </div>
        </div>

        {/* Title */}
        <h1
          id="coming-soon-title"
          className="mb-4 text-3xl font-bold tracking-tight text-white sm:text-4xl md:text-5xl"
        >
          {title}
        </h1>

        {/* Subtitle badge */}
        <p
          className="mb-6 inline-flex items-center gap-2 rounded-full bg-amber-500/10 px-4 py-2 text-amber-400"
          role="status"
        >
          <Hammer className="size-4" aria-hidden="true" />
          <span className="text-sm font-medium">Under Development</span>
        </p>

        {/* Description */}
        <p
          id="coming-soon-desc"
          className="mx-auto max-w-md text-base text-slate-400 sm:text-lg md:max-w-lg"
        >
          {description}
        </p>

        {/* Decorative elements - hidden from screen readers */}
        <div
          className="mt-10 flex items-center justify-center gap-2"
          aria-hidden="true"
        >
          <span className="h-px w-12 bg-linear-to-r from-transparent to-slate-600" />
          <Snowflake className="size-5 text-slate-600" aria-hidden="true" />
          <span className="h-px w-12 bg-linear-to-l from-transparent to-slate-600" />
        </div>

        {/* Progress dots animation - hidden from screen readers */}
        <div
          className="mt-6 flex items-center justify-center gap-1.5"
          aria-hidden="true"
        >
          <span
            className="size-2 animate-bounce rounded-full bg-amber-400/60"
            style={{ animationDelay: "0ms" }}
          />
          <span
            className="size-2 animate-bounce rounded-full bg-amber-400/60"
            style={{ animationDelay: "150ms" }}
          />
          <span
            className="size-2 animate-bounce rounded-full bg-amber-400/60"
            style={{ animationDelay: "300ms" }}
          />
        </div>
      </div>
    </section>
  );
}
