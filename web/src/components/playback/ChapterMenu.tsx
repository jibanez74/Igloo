import { ListOrdered, Check } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  MOTION_PLAYER_CHROME_PANEL_CLASS,
  PLAYER_ICON_BUTTON_CLASS,
} from "@/lib/constants";
import {
  chapterLabel,
  formatSpokenTime,
  formatTimecode,
  pluralize,
} from "@/lib/format";
import { trimmedOrNull } from "@/lib/nullable";
import { cn } from "@/lib/utils";
import type { PlaybackChapterType } from "@/types";

type ChapterMenuProps = {
  chapters: PlaybackChapterType[];
  currentTimeSec: number;
  onSelectChapter: (startTimeSec: number, title: string) => void;
  portalContainer?: HTMLElement | null;
};

// Runs on every render (~4x/sec while playing); a linear scan over at most a
// few dozen chapters is cheaper than any caching the React Compiler can't
// already do, so leave it unmemoized.
function getActiveChapterIndex(
  chapters: PlaybackChapterType[],
  currentTimeSec: number,
): number {
  for (let i = chapters.length - 1; i >= 0; i--) {
    if (currentTimeSec >= chapters[i].start_time) {
      return i;
    }
  }
  return -1;
}

export default function ChapterMenu({
  chapters,
  currentTimeSec,
  onSelectChapter,
  portalContainer,
}: ChapterMenuProps) {
  if (chapters.length === 0) return null;

  const activeIndex = getActiveChapterIndex(chapters, currentTimeSec);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className={cn(
          PLAYER_ICON_BUTTON_CLASS,
          "size-10 hover:bg-accent",
        )}
        aria-label={`Chapters, ${pluralize(chapters.length, "chapter")}`}
      >
        <ListOrdered className="size-5" aria-hidden="true" />
      </DropdownMenuTrigger>

      <DropdownMenuContent
        side="top"
        align="center"
        container={portalContainer}
        className={cn(MOTION_PLAYER_CHROME_PANEL_CLASS, "max-h-72 overflow-y-auto")}
      >
        {chapters.map((chapter, index) => {
          const isActive = index === activeIndex;
          const title = trimmedOrNull(chapter.title);
          const label = chapterLabel(chapter.title, index);
          // Build one spoken sentence so screen readers announce a logical
          // phrase ("Chapter 2 of 8, Opening Credits, starts at 1 minute 23
          // seconds, current chapter") instead of reading the raw timecode. The
          // named title is only added when the file actually provides one, to
          // avoid the redundant "Chapter 2 of 8, Chapter 2".
          const ariaLabel = [
            `Chapter ${index + 1} of ${chapters.length}`,
            title,
            `starts at ${formatSpokenTime(chapter.start_time)}`,
            isActive ? "current chapter" : null,
          ]
            .filter(Boolean)
            .join(", ");

          return (
            <DropdownMenuItem
              key={chapter.id}
              aria-label={ariaLabel}
              aria-current={isActive ? "true" : undefined}
              onSelect={() => onSelectChapter(chapter.start_time, label)}
            >
              {isActive ? (
                <Check className="size-4 text-primary" aria-hidden="true" />
              ) : (
                <span className="size-4" aria-hidden="true" />
              )}
              <span aria-hidden="true">
                {label}
                <span className="ml-2 text-muted-foreground">
                  {formatTimecode(chapter.start_time)}
                </span>
              </span>
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
