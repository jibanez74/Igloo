import type { ReactNode } from "react";
import SkeletonStatus from "@/components/shared/SkeletonStatus";
import {
  DETAIL_HERO_CONTENT_CLASS,
  DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
  DETAIL_HERO_SCRIM_FADE_CLASS,
  DETAIL_HERO_SHELL_CLASS,
  MOTION_LOADING_STATE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

// Hero placeholders sit on the skeleton's muted band; a foreground tint shows
// against it in both themes, where bg-background/50 vanished in light
// (design-system §3.4).
const HERO_PLACEHOLDER_CLASS = "bg-foreground/10";

type DetailSkeletonProps = {
  /** Spoken loading label, e.g. "Loading movie details". */
  label: string;
  /**
   * Mirrors the real hero: with actions it reserves the button row, without
   * them it takes the same extra bottom padding `DetailHero` applies, so
   * content arrival shifts nothing either way (design-system §3.4).
   */
  withActions: boolean;
  /** Placeholder geometry for sections below the overview, page-specific. */
  children?: ReactNode;
};

export default function DetailSkeleton({
  label,
  withActions,
  children,
}: DetailSkeletonProps) {
  return (
    <SkeletonStatus label={label} className={MOTION_LOADING_STATE_CLASS}>
      <div className={DETAIL_HERO_SHELL_CLASS}>
        <div className="absolute inset-0 bg-muted" />
        <div className={DETAIL_HERO_SCRIM_FADE_CLASS} />

        <div
          className={cn(
            DETAIL_HERO_CONTENT_CLASS,
            "space-y-4",
            !withActions && DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS,
          )}
        >
          <div className={cn("mx-auto aspect-2/3 w-28 rounded-lg sm:w-32 lg:hidden", HERO_PLACEHOLDER_CLASS)} />
          <div className={cn("mx-auto h-10 max-w-md rounded-sm lg:mx-0", HERO_PLACEHOLDER_CLASS)} />
          <div className={cn("mx-auto h-5 max-w-xs rounded-sm lg:mx-0", HERO_PLACEHOLDER_CLASS)} />
          <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
            <div className={cn("h-8 w-24 rounded-full", HERO_PLACEHOLDER_CLASS)} />
            <div className={cn("h-8 w-20 rounded-full", HERO_PLACEHOLDER_CLASS)} />
            <div className={cn("h-8 w-28 rounded-full", HERO_PLACEHOLDER_CLASS)} />
          </div>
          {withActions && (
            <div className="flex flex-wrap justify-center gap-2 lg:justify-start">
              <div className={cn("h-11 w-28 rounded-md", HERO_PLACEHOLDER_CLASS)} />
              <div className={cn("h-11 w-28 rounded-md", HERO_PLACEHOLDER_CLASS)} />
              <div className={cn("h-11 w-28 rounded-md", HERO_PLACEHOLDER_CLASS)} />
            </div>
          )}
        </div>
      </div>

      <div className="mt-6 space-y-2 text-left">
        <div className="h-4 w-full rounded-sm bg-muted" />
        <div className="h-4 w-full rounded-sm bg-muted" />
        <div className="h-4 w-3/4 rounded-sm bg-muted" />
      </div>

      {children}
    </SkeletonStatus>
  );
}
