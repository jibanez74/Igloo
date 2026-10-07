import { useState } from "react";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export type CrewDisclosureCredit = {
  key: string;
  job: string;
  department: string | null;
  name: string;
};

type CrewDisclosureProps = {
  credits: CrewDisclosureCredit[];
};

/**
 * "Show all crew" toggle under a key-crew summary. The expanded list grows
 * the page rather than scrolling inside it (the window is the one scroller,
 * design-system §3.1), laid out in columns so a long crew stays short.
 */
export default function CrewDisclosure({ credits }: CrewDisclosureProps) {
  const [expanded, setExpanded] = useState(false);

  if (credits.length === 0) return null;

  return (
    <div className="mt-4">
      <button
        type="button"
        aria-expanded={expanded}
        aria-controls="crew-full-list"
        onClick={() => setExpanded(v => !v)}
        className={cn(
          buttonVariants({ variant: "outline", size: "sm" }),
          "touch-manipulation",
        )}
      >
        {expanded ? "Show less" : "Show all crew"}
      </button>
      {expanded && (
        /* role kept because Tailwind's list-none strips list semantics in
           Safari. */
        <ul
          id="crew-full-list"
          role="list"
          aria-label={`Full crew list, ${credits.length} credits`}
          className="mt-3 grid list-none gap-x-6 gap-y-3 rounded-lg border border-primary/15 bg-card/40 p-3 sm:grid-cols-2 sm:px-4 lg:grid-cols-3"
        >
          {credits.map(credit => (
            <li key={credit.key}>
              <p className="text-sm">
                <span className="block text-muted-foreground">
                  {credit.job}
                  {credit.department ? (
                    <span className="text-muted-foreground">
                      {" "}
                      · {credit.department}
                    </span>
                  ) : null}
                </span>
                <span className="font-semibold text-foreground">
                  {credit.name}
                </span>
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
