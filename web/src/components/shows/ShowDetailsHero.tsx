import type { ReactNode } from "react";
import { Tv } from "lucide-react";
import DetailBackdrop from "@/components/shared/DetailBackdrop";
import ShowDetailsTitleHeading from "@/components/shows/ShowDetailsTitleHeading";
import ShowDetailsGenresList from "@/components/shows/ShowDetailsGenresList";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  DETAIL_HERO_CONTENT_CLASS,
  DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
  DETAIL_HERO_SHELL_CLASS,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { ShowGenreType } from "@/types";

type ShowDetailsHeroProps = {
  backdropUrl: string | null;
  posterUrl: string | null;
  name: string;
  premiereYear: number | null;
  firstAirDate: string | null;
  tagline: string | null;
  genres: ShowGenreType[];
  metadataSlot: ReactNode;
};

/**
 * The show hero has no actions slot: this page plays nothing, so it always
 * takes the no-actions bottom padding that keeps the last text line clear of
 * the hero's fade gradient.
 */
export default function ShowDetailsHero({
  backdropUrl,
  posterUrl,
  name,
  premiereYear,
  firstAirDate,
  tagline,
  genres,
  metadataSlot,
}: ShowDetailsHeroProps) {
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
            DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
          )}
        >
          <figure className="mx-auto mb-4 w-28 shrink-0 overflow-hidden rounded-lg border border-white/20 shadow-2xl shadow-black/40 sm:w-32 lg:hidden">
            {showPoster ? (
              <img
                src={posterUrl ?? ""}
                alt={`Poster for ${name}`}
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
                <Tv className="size-8 text-muted-foreground" aria-hidden="true" />
              </div>
            )}
          </figure>

          <ShowDetailsTitleHeading
            name={name}
            premiereYear={premiereYear}
            firstAirDate={firstAirDate}
          />

          {tagline && (
            <p className="mt-2 max-w-full text-base wrap-break-word text-white/85 italic drop-shadow-md sm:text-lg">
              <q>{tagline}</q>
            </p>
          )}

          {metadataSlot}

          <ShowDetailsGenresList genres={genres} />
        </div>
      </div>
    </header>
  );
}
