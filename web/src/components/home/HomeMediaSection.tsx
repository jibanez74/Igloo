import type { ReactNode } from "react";
import { Link, type LinkProps } from "@tanstack/react-router";
import { ChevronRight, type LucideIcon } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import EmptyState from "@/components/shared/EmptyState";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import ScrollRail from "@/components/shared/ScrollRail";
import SectionErrorAlert from "@/components/shared/SectionErrorAlert";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  MOTION_SECTION_ENTER_DELAYED_CLASS,
} from "@/lib/constants";
import { pluralize } from "@/lib/format";
import { cn } from "@/lib/utils";

type HomeMediaSectionProps<T> = {
  title: string;
  headingId: string;
  items: T[];
  isPending?: boolean;
  errorMessage: string | undefined;
  /** The query's refetch, rendered as Try again under the error. */
  onRetry?: () => void;
  loadingLabel?: string;
  emptyTitle: string;
  emptyDescription: string;
  emptyIcon: LucideIcon;
  /** Singular noun for the item count ("movie", "album") — pluralized here. */
  countNoun: string;
  /** The width of one card in the rail. */
  itemClassName: string;
  /** Where the whole of this row's library lives; drawn as "See all". */
  seeAllLink?: LinkProps;
  getKey: (item: T, index: number) => string;
  renderItem: (item: T, index: number) => ReactNode;
};

export default function HomeMediaSection<T>({
  title,
  headingId,
  items,
  isPending = false,
  errorMessage,
  onRetry,
  loadingLabel,
  emptyTitle,
  emptyDescription,
  emptyIcon: EmptyIcon,
  countNoun,
  itemClassName,
  seeAllLink,
  getKey,
  renderItem,
}: HomeMediaSectionProps<T>) {
  const sectionSummaryId = `${headingId}-summary`;
  const countLabel = pluralize(items.length, countNoun);
  let sectionSummary = "";

  if (isPending) {
    sectionSummary = loadingLabel ?? "";
  } else if (errorMessage) {
    sectionSummary = errorMessage;
  } else if (items.length > 0) {
    sectionSummary = `${countLabel} available in ${title.toLowerCase()}.`;
  } else {
    sectionSummary = emptyDescription;
  }

  const announcementMessage = isPending
    ? undefined
    : (errorMessage ?? (items.length === 0 ? emptyDescription : undefined));

  return (
    <section
      aria-labelledby={headingId}
      aria-describedby={sectionSummaryId}
      className={cn("mt-6 md:mt-8", MOTION_SECTION_ENTER_DELAYED_CLASS)}
    >
      <LiveAnnouncer message={announcementMessage} />

      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2
            id={headingId}
            className="text-xl font-semibold tracking-tight text-foreground md:text-2xl"
          >
            {title}
          </h2>
          {/* The section's aria-describedby text. It stays out of sight: the
              count pill, the error alert and the empty state already say the
              same thing where the eye lands, and the page read it four times
              over. */}
          <p id={sectionSummaryId} className="sr-only">
            {sectionSummary}
          </p>
        </div>

        {seeAllLink && !isPending && !errorMessage && items.length > 0 && (
          <Link
            {...seeAllLink}
            aria-label={`See all ${title.toLowerCase()}`}
            className={cn(
              "inline-flex shrink-0 items-center gap-0.5 rounded-sm text-sm font-medium text-muted-foreground hover:text-foreground",
              MOTION_MICRO_COLORS_CLASS,
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            See all
            <ChevronRight className="size-4" aria-hidden="true" />
          </Link>
        )}
      </div>

      {isPending ? (
        <div
          className="flex min-h-50 items-center justify-center py-12 sm:min-h-70"
          role="status"
          aria-label={loadingLabel}
        >
          <Spinner size="xl" tone="primary" />
        </div>
      ) : errorMessage ? (
        <SectionErrorAlert message={errorMessage} onRetry={onRetry} />
      ) : items.length > 0 ? (
        // One row that scrolls sideways, however many cards it holds. It is
        // still a list named by the heading: Orca names the row when focus
        // tabs into its first card, which it does not do for the region
        // alone. Focusable so the keyboard can scroll it.
        <ScrollRail label={title.toLowerCase()} asChild>
          <ul
            role="list"
            aria-labelledby={headingId}
            tabIndex={0}
            className={cn(
              "list-none gap-3 pb-3 sm:gap-4",
              FOCUS_VISIBLE_RING_CLASS,
            )}
          >
            {items.map((item, index) => (
              <li
                key={getKey(item, index)}
                className={cn("min-w-0 shrink-0", itemClassName)}
              >
                {renderItem(item, index)}
              </li>
            ))}
          </ul>
        </ScrollRail>
      ) : (
        <EmptyState
          icon={EmptyIcon}
          title={emptyTitle}
          description={emptyDescription}
          className="py-8 sm:py-10"
        />
      )}
    </section>
  );
}
