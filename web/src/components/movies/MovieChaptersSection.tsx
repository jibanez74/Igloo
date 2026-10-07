import { Link } from "@tanstack/react-router";
import ScrollRail from "@/components/shared/ScrollRail";
import { chapterLabel, formatTimecode } from "@/lib/format";
import {
  DETAIL_RAIL_HEADING_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { playbackSettingsToPlaySearch } from "@/lib/route-search";
import { cn } from "@/lib/utils";
import type { ChapterType } from "@/types/movies";
import type { PlaybackSettings } from "@/types/playback";

type MovieChaptersSectionProps = {
  chapters: ChapterType[];
  movieId: number;
  playbackSettings: PlaybackSettings;
};

export default function MovieChaptersSection({
  chapters,
  movieId,
  playbackSettings,
}: MovieChaptersSectionProps) {
  if (chapters.length === 0) return null;

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="chapters-heading">
      <h2
        id="chapters-heading"
        tabIndex={-1}
        className={DETAIL_RAIL_HEADING_CLASS}
      >
        Chapters
      </h2>
      <ScrollRail label="chapters" asChild>
        <ul
          className="snap-x snap-mandatory list-none gap-3 overscroll-x-contain pb-4 sm:gap-4"
          aria-label={`Chapters, ${chapters.length} total`}
        >
        {chapters.map((chapter, index) => (
          <li
            key={chapter.id}
            className="w-[min(18rem,calc(100vw-2.5rem))] shrink-0 snap-start scroll-ms-1 scroll-me-1 sm:scroll-ms-2 sm:scroll-me-2"
          >
            <Link
              to="/movies/$id/play"
              params={{ id: String(movieId) }}
              search={{
                ...playbackSettingsToPlaySearch(playbackSettings),
                start: chapter.start_time,
              }}
              className={cn(
                MOTION_MICRO_COLORS_CLASS,
                "flex min-h-13 touch-manipulation flex-col justify-center rounded-lg border border-primary/20 bg-muted/80 px-3 py-2.5 text-left text-sm text-primary",
                "hover:border-primary/40 hover:bg-muted",
                FOCUS_VISIBLE_RING_CLASS,
                "sm:min-h-0",
              )}
            >
              <span className="leading-snug font-medium">
                {chapterLabel(chapter.title, index)}
              </span>
              <span className="mt-0.5 text-muted-foreground">
                {formatTimecode(chapter.start_time)}
              </span>
            </Link>
          </li>
        ))}
        </ul>
      </ScrollRail>
    </section>
  );
}
