import { useQuery } from "@tanstack/react-query";
import { movieWatchProgressQueryOpts } from "@/lib/query-opts";
import { hasEligibleMovieResumeProgress } from "@/lib/movie-playback";
import { formatMinutesLeft } from "@/lib/format";

type MovieDetailsResumeProgressProps = {
  movieId: number;
};

/**
 * Thin watch-progress strip under the hero metadata (Netflix-style "X min
 * left"). Renders nothing when the movie is unwatched, finished, or marked
 * watched — Play behavior is unchanged (the in-player ResumeDialog still
 * offers resume vs start over).
 */
export default function MovieDetailsResumeProgress({
  movieId,
}: MovieDetailsResumeProgressProps) {
  const { data } = useQuery(movieWatchProgressQueryOpts(movieId));

  if (!data || data.error) return null;

  const { progress_sec, duration_sec, watched } = data.data;
  if (
    watched ||
    !hasEligibleMovieResumeProgress(progress_sec, duration_sec) ||
    progress_sec == null ||
    duration_sec == null
  ) {
    return null;
  }

  const progressPct = Math.min(
    100,
    Math.max(0, (progress_sec / duration_sec) * 100),
  );

  return (
    <div className="mx-auto mt-5 flex w-full max-w-md flex-col items-center gap-1.5 lg:mx-0 lg:items-start">
      <div
        className="h-1 w-full overflow-hidden rounded-full bg-white/25"
        aria-hidden="true"
      >
        <div
          className="h-full rounded-full bg-primary"
          style={{ width: `${progressPct}%` }}
        />
      </div>
      <p className="text-xs text-white/80">
        {formatMinutesLeft(progress_sec, duration_sec)}
      </p>
    </div>
  );
}
