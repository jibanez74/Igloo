import { Badge } from "@/components/ui/badge";
import { OVER_MEDIA_BADGE_CLASS } from "@/lib/constants";

/**
 * TMDB's community score as a quiet labelled badge, deliberately unlike the
 * tiered critic/audience rating chips: it is a different metric and must not
 * read as one of them (design-system §1.7 chip recipe for the spoken value).
 */
export default function TmdbScoreBadge({ score }: { score: number }) {
  return (
    <Badge variant="outline" className={OVER_MEDIA_BADGE_CLASS}>
      <span className="sr-only">
        {`TMDB user score: ${score.toFixed(1)} out of 10`}
      </span>
      <span aria-hidden="true" className="text-white/70">
        TMDB
      </span>
      <span aria-hidden="true" className="font-semibold">
        {score.toFixed(1)}
      </span>
    </Badge>
  );
}
