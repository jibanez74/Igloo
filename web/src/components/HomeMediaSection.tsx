import { Fragment, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import LiveAnnouncer from "@/components/LiveAnnouncer";
import { MOTION_SECTION_ENTER_DELAYED_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type HomeMediaSectionProps<T> = {
  title: string;
  headingId: string;
  items: T[];
  isPending: boolean;
  errorMessage: string | undefined;
  loadingLabel: string;
  emptyTitle: string;
  emptyDescription: string;
  emptyIcon: LucideIcon;
  countLabel: string;
  gridClassName: string;
  announcementMessage?: string;
  getKey: (item: T, index: number) => string;
  renderItem: (item: T, index: number) => ReactNode;
};

export default function HomeMediaSection<T>({
  title,
  headingId,
  items,
  isPending,
  errorMessage,
  loadingLabel,
  emptyTitle,
  emptyDescription,
  emptyIcon: EmptyIcon,
  countLabel,
  gridClassName,
  announcementMessage,
  getKey,
  renderItem,
}: HomeMediaSectionProps<T>) {
  const hasError = Boolean(errorMessage);
  const sectionSummaryId = `${headingId}-summary`;
  let sectionSummary = "";

  if (isPending) {
    sectionSummary = loadingLabel;
  } else if (hasError && errorMessage) {
    sectionSummary = errorMessage;
  } else if (items.length > 0) {
    sectionSummary = `${items.length} ${countLabel} available in ${title.toLowerCase()}.`;
  } else {
    sectionSummary = emptyDescription;
  }

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
            className="text-xl font-semibold tracking-tight text-white md:text-2xl"
          >
            {title}
          </h2>
          <p
            id={sectionSummaryId}
            className="mt-1 text-sm text-slate-400"
          >
            {sectionSummary}
          </p>
        </div>

        {!isPending && !hasError && items.length > 0 && (
          <p className="rounded-full border border-slate-700 bg-slate-950/60 px-3 py-1 text-xs font-medium text-slate-300">
            {items.length} {countLabel}
          </p>
        )}
      </div>

      {isPending ? (
        <div
          className="flex min-h-50 items-center justify-center py-12 sm:min-h-70"
          role="status"
          aria-label={loadingLabel}
        >
          <Spinner className="size-8 text-amber-400" />
        </div>
      ) : hasError ? (
        <Alert
          variant="destructive"
          className="border-red-500/20 bg-red-500/10 text-red-400"
        >
          <AlertCircle className="size-4" aria-hidden="true" />
          <AlertTitle>Error</AlertTitle>
          <AlertDescription>{errorMessage}</AlertDescription>
        </Alert>
      ) : items.length > 0 ? (
        <div className={gridClassName}>
          {items.map((item, index) => (
            <Fragment key={getKey(item, index)}>
              {renderItem(item, index)}
            </Fragment>
          ))}
        </div>
      ) : (
        <div className="py-12 text-center sm:py-16">
          <div className="mx-auto mb-4 flex size-16 items-center justify-center rounded-full border border-amber-500/20 bg-slate-800">
            <EmptyIcon className="size-6 text-amber-600" aria-hidden="true" />
          </div>
          <h3 className="mb-2 text-lg font-semibold text-slate-300">
            {emptyTitle}
          </h3>
          <p className="mx-auto max-w-md px-4 text-slate-400 sm:px-0">
            {emptyDescription}
          </p>
        </div>
      )}
    </section>
  );
}
