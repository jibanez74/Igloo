import { ListOrdered, Check } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { formatTimeSeconds } from "@/lib/format";
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
        className="flex size-10 items-center justify-center rounded-full text-slate-300 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:ring-cyan-400 focus:outline-none"
        aria-label={`Chapters, ${chapters.length} chapters`}
      >
        <ListOrdered className="size-5" aria-hidden="true" />
      </DropdownMenuTrigger>

      <DropdownMenuContent
        side="top"
        align="center"
        container={portalContainer}
        className="max-h-72 overflow-y-auto"
      >
        {chapters.map((chapter, index) => {
          const isActive = index === activeIndex;
          return (
            <DropdownMenuItem
              key={chapter.id}
              aria-current={isActive ? "true" : undefined}
              onSelect={() =>
                onSelectChapter(chapter.start_time, chapter.title)
              }
            >
              {isActive ? (
                <Check className="size-4 text-cyan-400" aria-hidden="true" />
              ) : (
                <span className="size-4" aria-hidden="true" />
              )}
              <span>
                {chapter.title}
                <span className="ml-2 text-muted-foreground">
                  {formatTimeSeconds(chapter.start_time)}
                </span>
              </span>
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
