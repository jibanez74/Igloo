import { FiYoutube } from "react-icons/fi";
import type { MovieExtras } from "@/types/MovieExtras";

type ExtrasSectionProps = {
  extras: MovieExtras[];
  onSelectVideo: (url: string) => void;
};

export default function ExtrasSection({
  extras,
  onSelectVideo,
}: ExtrasSectionProps) {
  const getYouTubeVideoId = (url: string) => {
    const regExp =
      /^.*(youtu.be\/|v\/|u\/\w\/|embed\/|watch\?v=|&v=)([^#&?]*).*/;

      const match = url.match(regExp);

    return match && match[2].length === 11 ? match[2] : null;
  };

  const handleKeyPress = (
    e: React.KeyboardEvent<HTMLButtonElement>,
    url: string
  ) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();

      onSelectVideo(url);
    }
  };

  return (
    <section className='container mx-auto px-4 py-12 border-t border-sky-200/10'>
      <h2 className='text-2xl font-bold text-white mb-6'>Extras</h2>

      <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6'>
        {extras.map(extra => {
          const videoId = getYouTubeVideoId(extra.url);

          const thumbnailUrl = videoId
            ? `https://img.youtube.com/vi/${videoId}/maxresdefault.jpg`
            : "";

          return (
            <button
              key={extra.id}
              onClick={() => onSelectVideo(extra.url)}
              onKeyDown={e => handleKeyPress(e, extra.url)}
              className='group relative aspect-video w-full rounded-xl overflow-hidden focus:outline-none 
                       focus:ring-2 focus:ring-sky-500/40 focus:ring-offset-2 focus:ring-offset-slate-800'
              aria-label={`Play ${extra.title}`}
            >
              {/* Thumbnail */}
              <img
                src={thumbnailUrl}
                alt={extra.title}
                className='w-full h-full object-cover transition-transform duration-300 
                         group-hover:scale-105'
                loading='lazy'
              />

              {/* Overlay */}
              <div
                className='absolute inset-0 bg-gradient-to-t from-slate-900 via-slate-900/50 to-transparent 
                          opacity-80 group-hover:opacity-90 transition-opacity'
              />

              {/* Play Button */}
              <div className='absolute inset-0 flex items-center justify-center'>
                <div
                  className='rounded-full bg-sky-500/90 p-4 transform transition-all duration-300 
                            group-hover:bg-sky-400 group-hover:scale-110'
                >
                  <FiYoutube
                    className='w-8 h-8 text-white'
                    aria-hidden='true'
                  />
                </div>
              </div>

              {/* Text Content */}
              <div className='absolute bottom-0 left-0 right-0 p-4'>
                <h3
                  className='text-base font-medium text-white line-clamp-2 mb-1 group-hover:text-sky-200 
                           transition-colors'
                >
                  {extra.title}
                </h3>
                <p className='text-sm text-sky-200/80 capitalize'>
                  {extra.kind}
                </p>
              </div>
            </button>
          );
        })}
      </div>
    </section>
  );
}
