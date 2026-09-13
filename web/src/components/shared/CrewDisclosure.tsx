import { useState } from "react";
import { buttonVariants } from "@/components/ui/button";
import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
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
 * "Show all crew" toggle under a key-crew summary. The expanded list is a
 * scroll container with no focusable content, so it is itself focusable and
 * carries the shared ring, or keyboard users could never scroll it.
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
        /* The list is the scroll container, focusable like the cast rail;
           role kept because Tailwind's list-none strips list semantics in
           Safari. */
        <ul
          id="crew-full-list"
          role="list"
          tabIndex={0}
          aria-label={`Full crew list, ${credits.length} credits`}
          className={cn(
            "mt-3 max-h-96 list-none space-y-3 overflow-y-auto rounded-lg border border-primary/15 bg-card/40 px-3 py-2 sm:px-4",
            FOCUS_VISIBLE_RING_CLASS,
          )}
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
