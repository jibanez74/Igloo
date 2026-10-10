import { Fragment, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import EmptyState from "@/components/shared/EmptyState";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import SectionErrorAlert from "@/components/shared/SectionErrorAlert";
import { MOTION_SECTION_ENTER_DELAYED_CLASS } from "@/lib/constants";
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
  gridClassName: string;
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
  gridClassName,
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

        {!isPending && !errorMessage && items.length > 0 && (
          <Badge variant="outline" className="px-3 py-1">
            {countLabel}
          </Badge>
        )}
      </div>

      {isPending ? (
        <div
          className="flex min-h-50 items-center justify-center py-12 sm:min-h-70"
          role="status"
          aria-label={loadingLabel}
        >
          <Spinner className="size-8 text-primary" />
        </div>
      ) : errorMessage ? (
        <SectionErrorAlert message={errorMessage} onRetry={onRetry} />
      ) : items.length > 0 ? (
        <div className={gridClassName}>
          {items.map((item, index) => (
            <Fragment key={getKey(item, index)}>
              {renderItem(item, index)}
            </Fragment>
          ))}
        </div>
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
