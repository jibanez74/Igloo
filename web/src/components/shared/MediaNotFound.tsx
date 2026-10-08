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

export default function MediaNotFound({
  title = "Error",
  message,
  back,
}: {
  /** The alert's heading; "Not found" when the resource is simply missing. */
  title?: string;
  message: string;
  back: BackDestination;
}) {
  const { label, ...linkProps } = BACK_DESTINATIONS[back];

  return (
    <div>
      <SectionErrorAlert title={title} message={message} />
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
