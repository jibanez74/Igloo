import { For } from "solid-js";
import { FiYoutube } from "solid-icons/fi";
import { getYouTubeVideoId, getYouTubeThumbnail } from "../utils/youtube";
import type { MovieExtras } from "../types/MovieExtras";


type ExtrasSectionProps = {
  extras: MovieExtras[];
  onSelectVideo: (url: string) => void;
};

export default function ExtrasSection(props: ExtrasSectionProps) {
  const handleKeyPress = (e: KeyboardEvent, url: string) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      props.onSelectVideo(url);
    }
  };

  const handleImageError = (event: Event) => {
    const img = event.target as HTMLImageElement;
    // Fall back to default thumbnail if maxresdefault fails
    img.src = img.src.replace('maxresdefault.jpg', 'hqdefault.jpg');
  };

  return (
    <section class="container mx-auto px-4 py-12 border-t border-sky-200/10">
      <h2 class="text-2xl font-bold text-white mb-6">Extras</h2>

      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        <For each={props.extras}>
          {(extra) => {
            const videoId = getYouTubeVideoId(extra.url);
            const thumbnailUrl = videoId ? getYouTubeThumbnail(videoId) : "";

            return (
              <button
                onClick={() => props.onSelectVideo(extra.url)}
                onKeyDown={(e) => handleKeyPress(e, extra.url)}
                class="group relative aspect-video w-full rounded-xl overflow-hidden focus:outline-none 
                       focus:ring-2 focus:ring-sky-500/40 focus:ring-offset-2 focus:ring-offset-slate-800"
                aria-label={`Play ${extra.title}`}
              >
                {/* Thumbnail */}
                <img
                  src={thumbnailUrl}
                  alt={extra.title}
                  class="w-full h-full object-cover transition-transform duration-300 
                         group-hover:scale-105"
                  loading="lazy"
                  onError={handleImageError}
                />

                {/* Overlay */}
                <div
                  class="absolute inset-0 bg-gradient-to-t from-slate-900 via-slate-900/50 to-transparent 
                          opacity-80 group-hover:opacity-90 transition-opacity"
                />

                {/* Play Button */}
                <div class="absolute inset-0 flex items-center justify-center">
                  <div
                    class="rounded-full bg-sky-500/90 p-4 transform transition-all duration-300 
                            group-hover:bg-sky-400 group-hover:scale-110"
                  >
                    <FiYoutube class="w-8 h-8 text-white" aria-hidden="true" />
                  </div>
                </div>

                {/* Text Content */}
                <div class="absolute bottom-0 left-0 right-0 p-4">
                  <h3
                    class="text-base font-medium text-white line-clamp-2 mb-1 group-hover:text-sky-200 
                           transition-colors"
                  >
                    {extra.title}
                  </h3>
                  <p class="text-sm text-sky-200/80 capitalize">{extra.kind}</p>
                </div>
              </button>
            );
          }}
        </For>
      </div>
    </section>
  );
}
