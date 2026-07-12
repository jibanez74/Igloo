import { useState } from "react";
import { buttonVariants } from "@/components/ui/button";
import { sortLibraryCrewForDisplay } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { MovieKeyCrewSectionProps } from "@/types";

const KEY_CREW_WRITERS_CAP = 3;

export default function MovieKeyCrewSection({ crew }: MovieKeyCrewSectionProps) {
  const [crewExpanded, setCrewExpanded] = useState(false);

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

  const hasMoreCrew = remainingCrew.length > 0;
  const showKeyCrewSummary = Boolean(director || writers.length > 0);
  const showCrewSection = crew.length > 0;

  if (!showCrewSection) return null;

  return (
    <section className="mt-6 text-left" aria-labelledby="crew-heading">
      <h2
        id="crew-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-foreground outline-none sm:text-xl"
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
      {hasMoreCrew && (
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
              aria-label={`Full crew list, ${remainingCrew.length} credits`}
              className="mt-3 max-h-96 overflow-y-auto rounded-lg border border-primary/15 bg-card/40 px-3 py-2 sm:px-4"
            >
              <ul className="list-none space-y-3">
                {remainingCrew.map(c => (
                  <li key={c.id}>
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
