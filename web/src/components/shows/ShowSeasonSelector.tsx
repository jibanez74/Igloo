import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { LIBRARY_TAB_TRIGGER_CLASS } from "@/lib/constants";
import { seasonLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ShowSeasonSummaryType } from "@/types";

type ShowSeasonSelectorProps = {
  seasons: ShowSeasonSummaryType[];
  selectedSeason: number;
  onSelectSeason: (seasonNumber: number) => void;
};

export default function ShowSeasonSelector({
  seasons,
  selectedSeason,
  onSelectSeason,
}: ShowSeasonSelectorProps) {
  if (seasons.length === 0) return null;

  return (
    <Tabs
      value={String(selectedSeason)}
      onValueChange={value => onSelectSeason(Number(value))}
    >
      {/* Scrolls horizontally rather than wrapping: a long-running show can
          carry twenty seasons, and a wrapped bar would push the list off screen. */}
      <TabsList
        className="h-auto max-w-full scrollbar-thin scrollbar-thumb-primary/50 justify-start overflow-x-auto"
        aria-label="Seasons"
      >
        {seasons.map(season => (
          <TabsTrigger
            key={season.id}
            value={String(season.season_number)}
            className={cn(LIBRARY_TAB_TRIGGER_CLASS, "flex-none")}
          >
            {seasonLabel(season.season_number)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
