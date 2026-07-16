import {
  DETAIL_HERO_CONTENT_CLASS,
  DETAIL_HERO_SCRIM_FADE_CLASS,
  DETAIL_HERO_SHELL_CLASS,
  MOTION_LOADING_STATE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

export default function MovieDetailsSkeleton() {
  return (
    <div
      className={MOTION_LOADING_STATE_CLASS}
      role="status"
      aria-label="Loading movie details"
    >
      <span className="sr-only">Loading movie details...</span>

      <div className={DETAIL_HERO_SHELL_CLASS} aria-hidden="true">
        <div className="absolute inset-0 bg-muted" />
        <div className={DETAIL_HERO_SCRIM_FADE_CLASS} />

        <div className={cn(DETAIL_HERO_CONTENT_CLASS, "space-y-4")}>
          <div className="mx-auto aspect-2/3 w-28 rounded-lg bg-background/50 sm:w-32 lg:hidden" />
          <div className="mx-auto h-10 max-w-md rounded-sm bg-background/50 lg:mx-0" />
          <div className="mx-auto h-5 max-w-xs rounded-sm bg-background/50 lg:mx-0" />
          <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
            <div className="h-8 w-24 rounded-full bg-background/50" />
            <div className="h-8 w-20 rounded-full bg-background/50" />
            <div className="h-8 w-28 rounded-full bg-background/50" />
          </div>
          <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
            <div className="h-11 w-28 rounded-md bg-background/50" />
            <div className="h-11 w-28 rounded-md bg-background/50" />
            <div className="h-11 w-28 rounded-md bg-background/50" />
          </div>
        </div>
      </div>

      <div className="mt-6 space-y-2 text-left" aria-hidden="true">
        <div className="h-4 w-full rounded-sm bg-muted" />
        <div className="h-4 w-full rounded-sm bg-muted" />
        <div className="h-4 w-3/4 rounded-sm bg-muted" />
      </div>
    </div>
  );
}
