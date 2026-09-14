import { useQuery } from "@tanstack/react-query";
import { movieWatchProgressQueryOpts } from "@/lib/query-opts";
import WatchProgressBar from "@/components/shared/WatchProgressBar";
import { hasEligibleResumeProgress } from "@/lib/video-playback";
import { formatTimeLeft } from "@/lib/format";

type MovieDetailsResumeProgressProps = {
  movieId: number;
};

/**
 * Thin watch-progress strip under the hero metadata (Netflix-style
 * "1 hr 35 min left"). Renders nothing when the movie is unwatched, finished,
 * or marked watched — Play behavior is unchanged (the in-player ResumeDialog
 * still offers resume vs start over).
 */
export default function MovieDetailsResumeProgress({
  movieId,
}: MovieDetailsResumeProgressProps) {
  const { data } = useQuery(movieWatchProgressQueryOpts(movieId));

  if (!data || data.error) return null;

  const { progress_sec, duration_sec, watched } = data.data;
  if (
    watched ||
    !hasEligibleResumeProgress(progress_sec, duration_sec) ||
    progress_sec == null ||
    duration_sec == null
  ) {
    return null;
  }

  const timeLeft = formatTimeLeft(progress_sec, duration_sec);

  return (
    <div className="mx-auto mt-5 flex w-full max-w-md flex-col items-center gap-1.5 lg:mx-0 lg:items-start">
      <WatchProgressBar
        progressSec={progress_sec}
        durationSec={duration_sec}
        trackClassName="bg-white/25"
      />
      <p className="text-xs text-white/80">
        <span className="sr-only">{timeLeft.spoken}</span>
        <span aria-hidden="true">{timeLeft.text}</span>
      </p>
    </div>
  );
}
