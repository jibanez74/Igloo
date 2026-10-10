import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import SectionErrorAlert from "@/components/shared/SectionErrorAlert";
import { buttonVariants } from "@/components/ui/button";
import {
  MOVIES_PLAYLISTS_TAB_SEARCH,
  MUSIC_PLAYLISTS_TAB_SEARCH,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

/**
 * Where a missing resource sends the reader, and what the link says. Prepared
 * rather than passed piecemeal: a `backTo`/`backLabel` pair let a caller link
 * to /music under the words "Back to Movies", and had nowhere to put the
 * search params the playlist pages need — which is why both of those pages
 * used to hand-roll their own not-found block instead.
 *
 * Add an entry when a call site needs one; keeping `to` a literal preserves
 * TanStack Router's typed navigation.
 */
const BACK_DESTINATIONS = {
  home: { to: "/", label: "Back to Home" },
  movies: { to: "/movies", label: "Back to Movies" },
  moviePlaylists: {
    to: "/movies",
    search: MOVIES_PLAYLISTS_TAB_SEARCH,
    label: "Back to movie playlists",
  },
  shows: { to: "/tv-shows", label: "Back to TV Shows" },
  music: { to: "/music", label: "Back to Music" },
  musicPlaylists: {
    to: "/music",
    search: MUSIC_PLAYLISTS_TAB_SEARCH,
    label: "Back to Playlists",
  },
} as const;

export type BackDestination = keyof typeof BACK_DESTINATIONS;

/**
 * A dead-end page: the page's own `h1` names what went wrong ("Movie not
 * found", "No access"), one alert sentence says why, and a link leads back.
 * The heading keeps the page navigable by headings and the alert carries no
 * title of its own, so the failure is not said twice.
 */
export default function MediaNotFound({
  heading,
  message,
  back,
  onRetry,
}: {
  heading: string;
  message: string;
  back: BackDestination;
  /** Try again inside the alert, for the one failure a retry can fix. */
  onRetry?: () => void;
}) {
  const { label, ...linkProps } = BACK_DESTINATIONS[back];

  return (
    <div>
      <h1 className="mb-6 text-3xl font-semibold tracking-tight text-foreground md:text-4xl">
        {heading}
      </h1>
      <SectionErrorAlert title={null} message={message} onRetry={onRetry} />
      <Link
        {...linkProps}
        className={cn(buttonVariants({ variant: "outline" }), "mt-4")}
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {label}
      </Link>
    </div>
  );
}
