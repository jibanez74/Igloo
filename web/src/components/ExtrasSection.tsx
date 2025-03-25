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
    img.src = img.src.replace("maxresdefault.jpg", "hqdefault.jpg");
  };

  return (
    <section class="container mx-auto px-4 py-12 border-t border-blue-900/20">
      <h2 class="text-2xl font-bold text-yellow-300 mb-6">Extras</h2>

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
                       focus:ring-2 focus:ring-yellow-300 focus:ring-offset-2 focus:ring-offset-blue-950
                       shadow-lg hover:shadow-2xl transition-all duration-300 hover:-translate-y-1"
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

                {/* Dark Overlay for Better Text Contrast */}
                <div
                  class="absolute inset-0 bg-gradient-to-t from-black via-black/50 to-transparent
                         opacity-75 group-hover:opacity-60 transition-opacity"
                />

                {/* Play Button */}
                <div class="absolute inset-0 flex items-center justify-center">
                  <div
                    class="rounded-full bg-yellow-300 p-4 transform transition-all duration-300 
                           group-hover:bg-yellow-400 group-hover:scale-110 
                           shadow-lg shadow-black/50 group-hover:shadow-xl"
                  >
                    <FiYoutube class="w-8 h-8 text-blue-950" aria-hidden="true" />
                  </div>
                </div>

                {/* Text Content */}
                <div class="absolute bottom-0 left-0 right-0 p-6 bg-gradient-to-t from-black to-transparent">
                  <h3
                    class="text-lg font-bold text-white line-clamp-2 mb-2 drop-shadow-lg
                           group-hover:text-yellow-300 transition-colors"
                  >
                    {extra.title}
                  </h3>
                  <div class="inline-block px-3 py-1 bg-blue-950/90 rounded-full">
                    <p class="text-sm font-medium text-yellow-300 capitalize">{extra.kind}</p>
                  </div>
                </div>
              </button>
            );
          }}
        </For>
      </div>
    </section>
  );
}
