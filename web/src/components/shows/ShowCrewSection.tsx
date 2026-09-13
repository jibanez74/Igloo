import { useState } from "react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
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
  const [crewExpanded, setCrewExpanded] = useState(false);

  if (creators.length === 0 && crew.length === 0) return null;

  return (
    <section className="mt-6 text-left" aria-labelledby="crew-heading">
      <h2
        id="crew-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-foreground outline-hidden sm:text-xl"
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

      {crew.length > 0 && (
        <div className="mt-4">
          <button
            type="button"
            aria-expanded={crewExpanded}
            aria-controls="crew-full-list"
            onClick={() => setCrewExpanded(v => !v)}
            className={cn(
              buttonVariants({ variant: "outline", size: "sm" }),
              "touch-manipulation",
            )}
          >
            {crewExpanded ? "Show less" : "Show all crew"}
          </button>
          {crewExpanded && (
            <div
              id="crew-full-list"
              aria-label={`Full crew list, ${crew.length} credits`}
              className="mt-3 max-h-96 overflow-y-auto rounded-lg border border-primary/15 bg-card/40 px-3 py-2 sm:px-4"
            >
              <ul className="list-none space-y-3">
                {crew.map(c => (
                  <li key={c.credit_id}>
                    <p className="text-sm">
                      <span className="block text-muted-foreground">
                        {c.job}
                        {c.department ? (
                          <span className="text-muted-foreground">
                            {" "}
                            · {c.department}
                          </span>
                        ) : null}
                      </span>
                      <span className="font-semibold text-foreground">
                        {c.artist_name}
                      </span>
                    </p>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
