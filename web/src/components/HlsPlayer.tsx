import { onMount, onCleanup, createSignal } from "solid-js";
import Hls from "hls.js";
import VideoLoadingOverlay from "./VideoLoadingOverlay";
import ErrorOverlay from "./ErrorOverlay";

type HlsPlayerProps = {
  src: string;
  title?: string;
  poster?: string;
  onError?: (error: Error) => void;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
};

export default function HlsPlayer(props: HlsPlayerProps) {
  let videoRef: HTMLVideoElement | undefined;
  let hls: Hls | undefined;

  const [error, setError] = createSignal<string | null>(null);
  const [isLoading, setIsLoading] = createSignal(true);

  onMount(() => {
    if (!videoRef) return;

    if (videoRef.canPlayType("application/vnd.apple.mpegurl")) {
      videoRef.src = props.src;
      return;
    }

    if (Hls.isSupported()) {
      hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true,
        backBufferLength: 90,
      });

      hls.attachMedia(videoRef);

      hls.on(Hls.Events.MEDIA_ATTACHED, () => {
        hls?.loadSource(props.src);
      });

      hls.on(Hls.Events.MANIFEST_LOADED, () => {
        setIsLoading(false);
      });

      hls.on(Hls.Events.ERROR, (_, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              setError("Network error occurred while loading the video");
              hls?.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              setError("Media error occurred while playing the video");
              hls?.recoverMediaError();
              break;
            default:
              setError("An error occurred while playing the video");
              break;
          }

          if (props.onError) {
            props.onError(new Error(data.details));
          }
        }
      });

      onCleanup(() => {
        if (hls) {
          hls.destroy();
          hls = undefined;
        }
      });
    } else {
      setError("HLS is not supported in your browser");

      if (props.onError) {
        props.onError(new Error("HLS not supported"));
      }
    }
  });

  return (
    <div class="relative rounded-lg overflow-hidden shadow-xl shadow-black/20 bg-slate-900">
      <video
        ref={videoRef}
        class="w-full h-full"
        poster={props.poster}
        title={props.title}
        autoplay={true}
        controls={true}
        onPlay={() => {
          setIsLoading(false);
          props.onPlay?.();
        }}
        onPause={props.onPause}
        onEnded={props.onEnded}
      />
      {isLoading() && props.poster && (
        <VideoLoadingOverlay poster={props.poster} title={props.title} />
      )}
      {error() && <ErrorOverlay message={error() || ""} />}
    </div>
  );
} 