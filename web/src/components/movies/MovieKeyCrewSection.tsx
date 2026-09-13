import CrewDisclosure from "@/components/shared/CrewDisclosure";
import { DETAIL_SECTION_HEADING_CLASS } from "@/lib/constants";
import { sortLibraryCrewForDisplay } from "@/lib/format";
import type { LibraryMovieCrewType } from "@/types/movies";

type MovieKeyCrewSectionProps = {
  crew: LibraryMovieCrewType[];
};

const KEY_CREW_WRITERS_CAP = 3;

export default function MovieKeyCrewSection({ crew }: MovieKeyCrewSectionProps) {
  const director = crew.find(c => c.job === "Director");
  const writers = crew
    .filter(c => c.department === "Writing")
    .slice(0, KEY_CREW_WRITERS_CAP);

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
        Key Crew
      </h2>
      {showKeyCrewSummary && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3">
          {director && (
            <p key={director.id} className="rounded-lg">
              <span className="block text-sm text-muted-foreground">Director</span>
              <span className="block font-semibold text-foreground">
                {director.artist_name}
              </span>
            </p>
          )}
          {writers.map(writer => (
            <p key={writer.id} className="rounded-lg">
              <span className="block text-sm text-muted-foreground">{writer.job}</span>
              <span className="block font-semibold text-foreground">
                {writer.artist_name}
              </span>
            </p>
          ))}
        </div>
      )}
      <CrewDisclosure
        credits={remainingCrew.map(c => ({
          key: String(c.id),
          job: c.job,
          department: c.department,
          name: c.artist_name,
        }))}
      />
    </section>
  );
}
