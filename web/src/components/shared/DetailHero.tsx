import type { ComponentType, ReactNode } from "react";
import type { LucideProps } from "lucide-react";
import DetailBackdrop from "@/components/shared/DetailBackdrop";
import DetailTitleHeading from "@/components/shared/DetailTitleHeading";
import DetailGenresList from "@/components/shared/DetailGenresList";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  DETAIL_HERO_CONTENT_CLASS,
  DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
  DETAIL_HERO_SHELL_CLASS,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type DetailHeroProps = {
  backdropUrl: string | null;
  posterUrl: string | null;
  posterAlt: string;
  /** Lucide icon shown in the poster box when there is no poster. */
  placeholderIcon: ComponentType<LucideProps>;
  titleId: string;
  title: string;
  year: number | null;
  /** Machine-readable date behind the year, when the catalog has one. */
  dateTime: string | null;
  tagline: string | null;
  genres: { id: number; tag: string }[];
  metadataSlot: ReactNode;
  progressSlot?: ReactNode;
  /**
   * Without an actions slot the content takes extra bottom padding so the
   * last text line stays clear of the hero's fade gradient.
   */
  actionsSlot?: ReactNode;
};

export default function DetailHero({
  backdropUrl,
  posterUrl,
  posterAlt,
  placeholderIcon: PlaceholderIcon,
  titleId,
  title,
  year,
  dateTime,
  tagline,
  genres,
  metadataSlot,
  progressSlot,
  actionsSlot,
}: DetailHeroProps) {
  const { showPoster, onError } = usePosterFallback(posterUrl ?? "");

  return (
    <header className={DETAIL_HERO_SHELL_CLASS}>
      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS, "absolute inset-0")}>
        <DetailBackdrop backdropUrl={backdropUrl} />
      </div>

      <div
        className={cn(
          DETAIL_PAGE_CONTENT_ENTER_CLASS,
          "delay-75 motion-reduce:delay-0",
        )}
      >
        <div
          className={cn(
            DETAIL_HERO_CONTENT_CLASS,
            !actionsSlot && DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
          )}
        >
          <figure className="mx-auto mb-4 w-28 shrink-0 overflow-hidden rounded-lg border border-white/20 shadow-2xl shadow-black/40 sm:w-32 lg:hidden">
            {showPoster ? (
              <img
                src={posterUrl ?? ""}
                alt={posterAlt}
                width={500}
                height={750}
                className="block aspect-2/3 w-full object-cover"
                onError={onError}
              />
            ) : (
              <div
                className="flex aspect-2/3 w-full items-center justify-center bg-muted"
                role="img"
                aria-label="No poster available"
              >
                <PlaceholderIcon
                  className="size-8 text-muted-foreground"
                  aria-hidden="true"
                />
              </div>
            )}
          </figure>

          <DetailTitleHeading
            id={titleId}
            title={title}
            year={year}
            dateTime={dateTime}
          />

          {tagline && (
            <p className="mt-2 max-w-full text-base wrap-break-word text-white/85 italic drop-shadow-md sm:text-lg">
              <q>{tagline}</q>
            </p>
          )}

          {metadataSlot}

          <DetailGenresList genres={genres} />

          {progressSlot}

          {actionsSlot}
        </div>
      </div>
    </header>
  );
}
