import { Snowflake, Hammer, type LucideIcon } from "lucide-react";
import {
  MOTION_DECORATIVE_BOUNCE_CLASS,
  MOTION_DECORATIVE_PING_CLASS,
  MOTION_PAGE_ENTER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

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
          focus:rounded-md focus:bg-muted focus:px-4 focus:py-2 focus:text-foreground focus:ring-2
          focus:ring-ring focus:outline-none"
        aria-label={announcement}
      >
        {title} - Under Development
      </span>

      <div className={MOTION_PAGE_ENTER_CLASS}>
        {/* Animated icon container */}
        <div className="relative mx-auto mb-8" aria-hidden="true">
          {/* Outer glow ring */}
          <div
            data-motion="decorative"
            className={cn(
              "absolute inset-0 rounded-full bg-primary/20",
              MOTION_DECORATIVE_PING_CLASS,
            )}
          />

          {/* Icon circle */}
          <div
            className="relative flex size-24 items-center justify-center rounded-full
              bg-linear-to-br from-muted to-muted shadow-xl
              ring-4 ring-border/50 sm:size-28 md:size-32"
          >
            <Icon
              className="size-10 text-primary sm:size-12 md:size-14"
              aria-hidden="true"
            />
          </div>
        </div>

        {/* Title */}
        <h1
          id="coming-soon-title"
          className="mb-4 text-3xl font-bold tracking-tight text-foreground sm:text-4xl md:text-5xl"
        >
          {title}
        </h1>

        {/* Subtitle badge */}
        <p
          className="mb-6 inline-flex items-center gap-2 rounded-full bg-primary/10 px-4 py-2 text-primary"
          role="status"
        >
          <Hammer className="size-4" aria-hidden="true" />
          <span className="text-sm font-medium">Under Development</span>
        </p>

        {/* Description */}
        <p
          id="coming-soon-desc"
          className="mx-auto max-w-md text-base text-muted-foreground sm:text-lg md:max-w-lg"
        >
          {description}
        </p>

        {/* Decorative elements - hidden from screen readers */}
        <div
          className="mt-10 flex items-center justify-center gap-2"
          aria-hidden="true"
        >
          <span className="h-px w-12 bg-linear-to-r from-transparent to-muted" />
          <Snowflake className="size-5 text-muted-foreground" aria-hidden="true" />
          <span className="h-px w-12 bg-linear-to-l from-transparent to-muted" />
        </div>

        {/* Progress dots animation - hidden from screen readers */}
        <div
          className="mt-6 flex items-center justify-center gap-1.5"
          aria-hidden="true"
        >
          <span
            data-motion="decorative"
            className={cn(
              "size-2 rounded-full bg-primary/60",
              MOTION_DECORATIVE_BOUNCE_CLASS,
            )}
            style={{ animationDelay: "0ms" }}
          />
          <span
            data-motion="decorative"
            className={cn(
              "size-2 rounded-full bg-primary/60",
              MOTION_DECORATIVE_BOUNCE_CLASS,
            )}
            style={{ animationDelay: "150ms" }}
          />
          <span
            data-motion="decorative"
            className={cn(
              "size-2 rounded-full bg-primary/60",
              MOTION_DECORATIVE_BOUNCE_CLASS,
            )}
            style={{ animationDelay: "300ms" }}
          />
        </div>
      </div>
    </section>
  );
}
