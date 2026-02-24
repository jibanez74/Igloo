import { Link } from "@tanstack/react-router";
import { Play } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

type MovieTrailerButtonProps = {
  movieId: number;
  returnTo: string;
  returnToParams?: { id: string };
};

export default function MovieTrailerButton({
  movieId,
  returnTo,
  returnToParams,
}: MovieTrailerButtonProps) {
  return (
    <Link
      to="/trailer"
      search={{
        mediaType: "movie",
        mediaId: movieId,
        returnTo,
      }}
      mask={returnToParams ? { to: returnTo, params: returnToParams } : undefined}
      className={`${buttonVariants({ variant: "accent-pill", size: "lg" })} mt-6 inline-flex min-h-11 items-center gap-2 px-6 py-3`}
    >
      <Play className="size-4 fill-current" aria-hidden="true" />
      Play Trailer
    </Link>
  );
}
