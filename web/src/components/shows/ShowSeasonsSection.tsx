import ShowSeasonEpisodeList from "@/components/shows/ShowSeasonEpisodeList";
import ShowSeasonSelector from "@/components/shows/ShowSeasonSelector";
import type { ShowSeasonSummaryType } from "@/types";

type ShowSeasonsSectionProps = {
  showId: number;
  seasons: ShowSeasonSummaryType[];
  selectedSeason: number;
  onSelectSeason: (seasonNumber: number) => void;
};

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
        className="mb-4 text-xl font-semibold text-foreground outline-hidden sm:text-2xl"
      >
        Seasons
      </h2>

      <ShowSeasonSelector
        seasons={seasons}
        selectedSeason={selectedSeason}
        onSelectSeason={onSelectSeason}
      />

      <ShowSeasonEpisodeList showId={showId} seasonNumber={selectedSeason} />
    </section>
  );
}
