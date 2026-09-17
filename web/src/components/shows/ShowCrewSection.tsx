import CrewDisclosure from "@/components/shared/CrewDisclosure";
import { DETAIL_SECTION_HEADING_CLASS } from "@/lib/constants";
import type { ShowCrewCreditType, ShowPersonType } from "@/types";

type ShowCrewSectionProps = {
  creators: ShowPersonType[];
  crew: ShowCrewCreditType[];
};

/** Creators are billed first, then the aggregate crew behind a disclosure —
 *  the show analogue of the movie key-crew section. */
export default function ShowCrewSection({
  creators,
  crew,
}: ShowCrewSectionProps) {
  if (creators.length === 0 && crew.length === 0) return null;

  return (
    <section className="mt-6 text-left" aria-labelledby="crew-heading">
      <h2
        id="crew-heading"
        tabIndex={-1}
        className={DETAIL_SECTION_HEADING_CLASS}
      >
        Key Crew
      </h2>

      {creators.length > 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 md:grid-cols-3">
          {creators.map(creator => (
            <p key={creator.id} className="rounded-lg">
              <span className="block text-sm text-muted-foreground">
                Creator
              </span>
              <span className="block font-semibold text-foreground">
                {creator.name}
              </span>
            </p>
          ))}
        </div>
      )}

      <CrewDisclosure
        credits={crew.map(c => ({
          key: c.credit_id,
          job: c.job,
          department: c.department,
          name: c.artist_name,
        }))}
      />
    </section>
  );
}
