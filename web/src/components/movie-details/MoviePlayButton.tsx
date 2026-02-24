import { Link } from "@tanstack/react-router";
import { Play } from "lucide-react";
import { buttonVariants } from "@/components/ui/button";

type MoviePlayButtonProps = {
  movieId: number;
};

export default function MoviePlayButton({ movieId }: MoviePlayButtonProps) {
  return (
    <Link
      to="/movies/$id/play"
      params={{ id: String(movieId) }}
      className={`${buttonVariants({ variant: "accent-pill", size: "lg" })} inline-flex min-h-11 items-center gap-2 px-6 py-3`}
    >
      <Play className="size-4 fill-current" aria-hidden="true" />
      Play
    </Link>
  );
}
