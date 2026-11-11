import PersonLink from "@/components/shared/PersonLink";
import CrewDisclosure from "@/components/shared/CrewDisclosure";
import { DETAIL_SECTION_HEADING_CLASS } from "@/lib/constants";
import { crewSectionLabels, movieKeyCrew } from "@/lib/crew";
import { sortLibraryCrewForDisplay } from "@/lib/format";
import type { LibraryMovieCrewType } from "@/types/movies";

type MovieKeyCrewSectionProps = {
  crew: LibraryMovieCrewType[];
};

export default function MovieKeyCrewSection({ crew }: MovieKeyCrewSectionProps) {
  const { director, writers } = movieKeyCrew(crew);

  const keyCrewRowIds = new Set<number>();
  if (director) keyCrewRowIds.add(director.id);
  for (const w of writers) keyCrewRowIds.add(w.id);

  const remainingCrew = crew
    .filter(c => !keyCrewRowIds.has(c.id))
    .sort(sortLibraryCrewForDisplay);

  const showKeyCrewSummary = Boolean(director || writers.length > 0);

  if (crew.length === 0) return null;

  return (
    <section className="mt-6 text-left" aria-labelledby="crew-heading">
      <h2
        id="crew-heading"
        tabIndex={-1}
        className={DETAIL_SECTION_HEADING_CLASS}
      >
        {crewSectionLabels(showKeyCrewSummary).heading}
      </h2>
      {showKeyCrewSummary && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3">
          {director && (
            <p key={director.id} className="rounded-lg">
              <span className="block text-sm text-muted-foreground">Director</span>
              <PersonLink personId={director.tmdb_id} className="block font-semibold text-foreground">
                {director.artist_name}
              </PersonLink>
            </p>
          )}
          {writers.map(writer => (
            <p key={writer.id} className="rounded-lg">
              <span className="block text-sm text-muted-foreground">{writer.job}</span>
              <PersonLink personId={writer.tmdb_id} className="block font-semibold text-foreground">
                {writer.artist_name}
              </PersonLink>
            </p>
          ))}
        </div>
      )}
      <CrewDisclosure
        credits={remainingCrew.map(c => ({
          key: String(c.id),
          personId: c.tmdb_id,
          job: c.job,
          department: c.department,
          name: c.artist_name,
        }))}
      />
    </section>
  );
}
