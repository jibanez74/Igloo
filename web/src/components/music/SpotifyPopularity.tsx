import {
  SPOTIFY_BRAND_FILL_CLASS,
  SPOTIFY_BRAND_ICON_CLASS,
  SPOTIFY_BRAND_TEXT_CLASS,
  MOTION_PROGRESS_FILL_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

export function SpotifyGlyph({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="currentColor"
      aria-hidden="true"
    >
      <path d="M12 0C5.4 0 0 5.4 0 12s5.4 12 12 12 12-5.4 12-12S18.66 0 12 0zm5.521 17.34c-.24.359-.66.48-1.021.24-2.82-1.74-6.36-2.101-10.561-1.141-.418.122-.779-.179-.899-.539-.12-.421.18-.78.54-.9 4.56-1.021 8.52-.6 11.64 1.32.42.18.479.659.301 1.02zm1.44-3.3c-.301.42-.841.6-1.262.3-3.239-1.98-8.159-2.58-11.939-1.38-.479.12-1.02-.12-1.14-.6-.12-.48.12-1.021.6-1.141C9.6 9.9 15 10.561 18.72 12.84c.361.181.54.78.241 1.2zm.12-3.36C15.24 8.4 8.82 8.16 5.16 9.301c-.6.179-1.2-.181-1.38-.721-.18-.601.18-1.2.72-1.381 4.26-1.26 11.28-1.02 15.721 1.621.539.3.719 1.02.419 1.56-.299.421-1.02.599-1.559.3z" />
    </svg>
  );
}

export function SpotifyPopularityMeter({ score }: { score: number }) {
  const pct = Math.max(0, Math.min(100, Math.round(score)));
  return (
    <div
      className="mx-auto mt-4 w-full max-w-md lg:mx-0"
      role="group"
      aria-label={`Spotify popularity ${pct} out of 100`}
    >
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
          <SpotifyGlyph className={cn("size-4 shrink-0", SPOTIFY_BRAND_ICON_CLASS)} />
          <span>Spotify popularity</span>
        </span>
        <span
          className={cn(
            "shrink-0 font-semibold tabular-nums",
            SPOTIFY_BRAND_TEXT_CLASS,
          )}
        >
          {pct}
        </span>
      </div>
      <div
        className="mt-2 h-2 overflow-hidden rounded-full bg-accent"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className={cn(
            "h-full rounded-full",
            SPOTIFY_BRAND_FILL_CLASS,
            MOTION_PROGRESS_FILL_CLASS,
          )}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
