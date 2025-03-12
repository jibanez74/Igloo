import { getYouTubeVideoId } from "../utils/youtube";

type YoutubePlayerProps = {
  url: string;
  title?: string;
  autoplay?: boolean;
  class?: string;
  onEnded?: () => void;
};

export default function YoutubePlayer(props: YoutubePlayerProps) {
  const videoId = getYouTubeVideoId(props.url);
  if (!videoId) return null;

  const params = new URLSearchParams({
    rel: "0",
    modestbranding: "1",
    ...(props.autoplay && { autoplay: "1" }),
    ...(props.onEnded && { enablejsapi: "1" }),
  });

  return (
    <div
      class={`aspect-video bg-slate-800 rounded-lg overflow-hidden ${props.class || ""}`}
    >
      <iframe
        src={`https://www.youtube.com/embed/${videoId}?${params.toString()}`}
        title={props.title || "YouTube video player"}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
        allowfullscreen
        class="w-full h-full"
        onEnded={props.onEnded}
      />
    </div>
  );
}
