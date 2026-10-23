import { SpotifyGlyph } from "@/components/music/SpotifyPopularity";
import { FOCUS_VISIBLE_RING_CLASS, SPOTIFY_BRAND_ICON_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

/**
 * An "Open on Spotify" link for a matched album or musician. Like the people
 * page's external links, its accessible name says what it opens and that it
 * opens in a new tab.
 */
export default function SpotifyLink({
  href,
  name,
  className,
}: {
  href: string;
  name: string;
  className?: string;
}) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        "inline-flex items-center gap-1.5 rounded-sm text-sm text-primary underline",
        FOCUS_VISIBLE_RING_CLASS,
        className,
      )}
      aria-label={`${name} on Spotify (opens in a new tab)`}
    >
      <SpotifyGlyph className={cn("size-4 shrink-0", SPOTIFY_BRAND_ICON_CLASS)} />
      Open on Spotify
    </a>
  );
}
