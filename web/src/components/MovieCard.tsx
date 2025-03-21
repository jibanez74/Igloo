import { Link } from "@tanstack/solid-router";
import getImgSrc from "../utils/getImgSrc";
import type { SimpleMovie } from "../types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
  imgLoading: "eager" | "lazy";
};

export default function MovieCard(props: MovieCardProps) {
  const imgSrc = getImgSrc(props.movie.thumb);

  return (
    <Link
      to="/movies/$movieID"
      resetScroll={true}
      params={{
        movieID: props.movie.id.toString(),
      }}
      preload="intent"
    >
      <div class="group relative bg-blue-950/50 backdrop-blur-sm rounded-xl overflow-hidden shadow-lg shadow-blue-900/20 transition-all duration-300 hover:shadow-yellow-300/20 hover:scale-[1.02]">
        <img
          src={imgSrc}
          alt={`${props.movie.title}`}
          class="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105 group-hover:brightness-[0.3]"
          loading={props.imgLoading}
        />

        <div class="absolute inset-0 bg-gradient-to-t from-blue-950 via-blue-950/50 to-transparent opacity-50 transition-opacity duration-300 group-hover:opacity-90" />

        <div class="absolute inset-0 p-4 flex flex-col justify-end transition-all duration-300">
          <div class="transform transition-all duration-300 group-hover:translate-y-0">
            <h3 class="text-lg font-semibold text-white mb-2">
              <span class="line-clamp-1 group-hover:line-clamp-none transition-all duration-300">
                {props.movie.title}
              </span>
            </h3>

            <p class="text-sm text-yellow-300/90 transition-opacity duration-300 group-hover:opacity-100">
              {props.movie.year}
            </p>
          </div>
        </div>
      </div>
    </Link>
  );
}
