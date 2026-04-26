import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Spinner } from "@/components/ui/spinner";
import LiveAnnouncer from "@/components/LiveAnnouncer";

type HomeMediaSectionProps<T> = {
  title: string;
  headingId: string;
  ariaLabel: string;
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
  renderItem: (item: T) => ReactNode;
};

export default function HomeMediaSection<T>({
  title,
  headingId,
  ariaLabel,
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
  renderItem,
}: HomeMediaSectionProps<T>) {
  const hasError = Boolean(errorMessage);

  return (
    <section
      role="region"
      aria-labelledby={headingId}
      aria-label={ariaLabel}
      className="mt-6 md:mt-8"
    >
      {announcementMessage && <LiveAnnouncer message={announcementMessage} />}

      <h2
        id={headingId}
        className="mb-4 text-xl font-semibold tracking-tight text-white md:text-2xl"
      >
        {title}
      </h2>

      {isPending ? (
        <div
          className="flex min-h-50 items-center justify-center py-12 sm:min-h-70"
          role="status"
          aria-label={loadingLabel}
        >
          <Spinner className="size-8 text-amber-400" />
          <span className="sr-only">{loadingLabel}</span>
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
        <>
          <span
            tabIndex={0}
            className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:rounded-md focus:bg-amber-400 focus:px-4 focus:py-2 focus:text-slate-900"
            aria-label={`${title} section, ${items.length} ${countLabel}`}
          >
            {title} - {items.length} {countLabel}
          </span>
          <div className={gridClassName}>{items.map(renderItem)}</div>
        </>
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
