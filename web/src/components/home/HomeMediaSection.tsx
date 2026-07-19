import { Fragment, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import EmptyState from "@/components/shared/EmptyState";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import SectionErrorAlert from "@/components/shared/SectionErrorAlert";
import { MOTION_SECTION_ENTER_DELAYED_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type HomeMediaSectionProps<T> = {
  title: string;
  headingId: string;
  items: T[];
  isPending?: boolean;
  errorMessage: string | undefined;
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
  const countLabel = `${items.length} ${countNoun}${items.length === 1 ? "" : "s"}`;
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
      role="region"
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
          <p
            id={sectionSummaryId}
            className="mt-1 text-sm text-muted-foreground"
          >
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
        <SectionErrorAlert message={errorMessage} />
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
        />
      )}
    </section>
  );
}
