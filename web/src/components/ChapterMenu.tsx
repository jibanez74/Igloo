import { ListOrdered, Check } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";
import { formatSpokenTime, formatTimecode } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ChapterType } from "@/types";

type ChapterMenuProps = {
  chapters: ChapterType[];
  currentTimeSec: number;
  onSelectChapter: (startTimeSec: number, title: string) => void;
  portalContainer?: HTMLElement | null;
};

function getActiveChapterIndex(
  chapters: ChapterType[],
  currentTimeSec: number,
): number {
  for (let i = chapters.length - 1; i >= 0; i--) {
    if (currentTimeSec >= chapters[i].start_time) {
      return i;
    }
  }
  return -1;
}

// Chapter titles come straight from the media file's metadata and are often
// blank or whitespace. When present we use the title; otherwise fall back to a
// human-readable "Chapter N" label so every entry has a meaningful name on
// screen and for screen readers.
function getChapterTitle(chapter: ChapterType): string {
  return chapter.title?.trim() ?? "";
}

function getChapterLabel(chapter: ChapterType, index: number): string {
  return getChapterTitle(chapter) || `Chapter ${index + 1}`;
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
          MOTION_PLAYER_CHROME_BUTTON_CLASS,
          "flex size-10 items-center justify-center rounded-full text-slate-300 hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none",
        )}
        aria-label={`Chapters, ${chapters.length} ${
          chapters.length === 1 ? "chapter" : "chapters"
        }`}
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
          const title = getChapterTitle(chapter);
          const label = getChapterLabel(chapter, index);
          // Build one spoken sentence so screen readers announce a logical
          // phrase ("Chapter 2 of 8, Opening Credits, starts at 1 minute 23
          // seconds, current chapter") instead of reading the raw timecode. The
          // named title is only added when the file actually provides one, to
          // avoid the redundant "Chapter 2 of 8, Chapter 2".
          const ariaLabel = [
            `Chapter ${index + 1} of ${chapters.length}`,
            title || null,
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
                <Check className="size-4 text-cyan-400" aria-hidden="true" />
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
