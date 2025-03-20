import Spinner from "./Spinner";

type VideoLoadingOverlayProps = {
  poster?: string;
  title?: string;
};

export default function VideoLoadingOverlay(props: VideoLoadingOverlayProps) {
  return (
    <div class="absolute inset-0 bg-blue-950/90 backdrop-blur-sm">
      <div class="absolute inset-0 flex items-center justify-center">
        <div class="relative w-48 aspect-[2/3] rounded-lg overflow-hidden ring-1 ring-blue-800/20">
          <img
            src={props.poster}
            alt={props.title || "Loading"}
            class="w-full h-full object-cover"
          />
          <div class="absolute inset-0 bg-gradient-to-t from-blue-950/90 via-blue-950/40 to-transparent" />
          <div class="absolute inset-x-0 bottom-0 p-4 text-center">
            <div class="mx-auto mb-2">
              <Spinner size="lg" />
            </div>
            <p class="text-yellow-300 text-sm font-medium">Loading</p>
          </div>
        </div>
      </div>
    </div>
  );
} 