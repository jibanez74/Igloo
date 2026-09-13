import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import ShowSeasonEpisodeList, {
  EpisodeRowsPlaceholder,
} from "@/components/shows/ShowSeasonEpisodeList";
import {
  DETAIL_RAIL_HEADING_CLASS,
  LIBRARY_TAB_TRIGGER_CLASS,
} from "@/lib/constants";
import { seasonLabel } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { ShowSeasonSummaryType } from "@/types";

type ShowSeasonsSectionProps = {
  showId: number;
  seasons: ShowSeasonSummaryType[];
  selectedSeason: number;
  onSelectSeason: (seasonNumber: number) => void;
};

// The tab strip scrolls on the same viewport bleed as the cast and extras
// rails (design-system §3.2) rather than wrapping: a long-running show can
// carry twenty seasons, and a wrapped bar would push the list off screen.
const SEASON_STRIP_CLASS =
  "-mx-4 overflow-x-auto px-4 pb-1 scrollbar-thin scrollbar-thumb-primary/50 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8";

export default function ShowSeasonsSection({
  showId,
  seasons,
  selectedSeason,
  onSelectSeason,
}: ShowSeasonsSectionProps) {
  if (seasons.length === 0) return null;

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="seasons-heading">
      <h2
        id="seasons-heading"
        tabIndex={-1}
        className={DETAIL_RAIL_HEADING_CLASS}
      >
        Seasons
      </h2>

      {/* A full Tabs pair: the episode list is the tabpanel every season tab
          points at, so the tab/panel contract holds for screen readers. */}
      <Tabs
        value={String(selectedSeason)}
        onValueChange={value => onSelectSeason(Number(value))}
      >
        <div className={SEASON_STRIP_CLASS}>
          <TabsList className="h-auto w-max justify-start" aria-label="Seasons">
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
        </div>

        <TabsContent value={String(selectedSeason)}>
          <ShowSeasonEpisodeList
            showId={showId}
            seasonNumber={selectedSeason}
          />
        </TabsContent>
      </Tabs>
    </section>
  );
}

/** Page-skeleton geometry for this section: heading, tab strip, three rows. */
export function ShowSeasonsSectionPlaceholder() {
  return (
    <div className="mt-8 space-y-4 sm:mt-10" aria-hidden="true">
      <div className="h-7 w-32 rounded-sm bg-muted sm:h-8" />
      <div className="h-12 w-72 max-w-full rounded-lg bg-muted" />
      <div className="mt-4">
        <EpisodeRowsPlaceholder />
      </div>
    </div>
  );
}
