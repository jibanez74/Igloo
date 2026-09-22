import { Link } from "@tanstack/react-router";
import { AlertCircle, ArrowLeft } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { buttonVariants } from "@/components/ui/button";
import { MOVIES_PLAYLISTS_TAB_SEARCH } from "@/lib/constants";
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
    search: { tab: "playlists" as const },
    label: "Back to Playlists",
  },
} as const;

export type BackDestination = keyof typeof BACK_DESTINATIONS;

export default function MediaNotFound({
  message,
  back,
}: {
  message: string;
  back: BackDestination;
}) {
  const { label, ...linkProps } = BACK_DESTINATIONS[back];

  return (
    <div>
      <Alert className="border-destructive/20 bg-destructive/10 text-destructive">
        <AlertCircle className="size-4" aria-hidden="true" />
        <AlertTitle>Error</AlertTitle>
        <AlertDescription>{message}</AlertDescription>
      </Alert>
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
