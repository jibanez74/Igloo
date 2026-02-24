import { useState } from "react";
import { Heart } from "lucide-react";
import { Button } from "@/components/ui/button";

type MovieLikeButtonProps = {
  movieId: number;
  defaultLiked?: boolean;
};

export default function MovieLikeButton({
  movieId: _movieId,
  defaultLiked = false,
}: MovieLikeButtonProps) {
  const [liked, setLiked] = useState(defaultLiked);

  return (
    <Button
      type="button"
      variant="outline"
      size="lg"
      className="min-h-11 rounded-full border-slate-600 bg-slate-700 px-6 py-3 font-semibold text-white transition-colors hover:border-slate-500 hover:bg-slate-600 focus-visible:ring-amber-400/50"
      aria-label={liked ? "Remove from likes" : "Add to likes"}
      onClick={() => setLiked(prev => !prev)}
    >
      <Heart
        className={`size-4 ${liked ? "fill-red-400 text-red-400" : ""}`}
        aria-hidden="true"
      />
      {liked ? "Liked" : "Like"}
    </Button>
  );
}
