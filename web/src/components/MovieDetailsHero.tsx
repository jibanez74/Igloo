import { Film } from "lucide-react";
import MovieDetailsBackdrop from "@/components/MovieDetailsBackdrop";
import MovieDetailsTitleHeading from "@/components/MovieDetailsTitleHeading";
import MovieDetailsGenresList from "@/components/MovieDetailsGenresList";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  DETAIL_HERO_CONTENT_CLASS,
  DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
  DETAIL_HERO_SHELL_CLASS,
  DETAIL_PAGE_CONTENT_ENTER_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { MovieDetailsHeroProps } from "@/types";

export default function MovieDetailsHero({
  backdropUrl,
  posterUrl,
  movieTitle,
  releaseYear,
  releaseDateStr,
  tagLine,
  genres,
  metadataSlot,
  progressSlot,
  actionsSlot,
}: MovieDetailsHeroProps) {
  const { showPoster, onError } = usePosterFallback(posterUrl ?? "");

  return (
    <header className={DETAIL_HERO_SHELL_CLASS}>
      <div className={cn(DETAIL_PAGE_CONTENT_ENTER_CLASS, "absolute inset-0")}>
        <MovieDetailsBackdrop backdropUrl={backdropUrl} />
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
                alt={`Movie poster for ${movieTitle}`}
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
                <Film
                  className="size-8 text-muted-foreground"
                  aria-hidden="true"
                />
              </div>
            )}
          </figure>

          <MovieDetailsTitleHeading
            title={movieTitle}
            releaseYear={releaseYear}
            releaseDateStr={releaseDateStr}
          />

          {tagLine && (
            <p className="mt-2 max-w-full text-base wrap-break-word text-white/85 italic drop-shadow-md sm:text-lg">
              <q>{tagLine}</q>
            </p>
          )}

          {metadataSlot}

          <MovieDetailsGenresList genres={genres} />

          {progressSlot}

          {actionsSlot}
        </div>
      </div>
    </header>
  );
}
