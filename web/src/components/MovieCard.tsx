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
      to='/movies/$movieID'
      params={{
        movieID: props.movie.id.toString(),
      }}
    >
      <div class='group relative bg-slate-900/50 backdrop-blur-sm rounded-xl overflow-hidden shadow-lg shadow-blue-900/20 transition-all duration-300 hover:shadow-blue-500/20 hover:scale-[1.02]'>
        <img
          src={imgSrc}
          alt={`${props.movie.title}`}
          class='w-full h-full object-cover transition-transform duration-300 group-hover:scale-105'
          loading={props.imgLoading}
        />

        <div class='absolute inset-0 bg-gradient-to-t from-slate-900 via-transparent to-transparent opacity-50' />

        <div class='p-4 space-y-4'>
          <div>
            <h3
              class='text-lg font-semibold text-white line-clamp-1 group/title relative'
              title={props.movie.title}
            >
              <span class='absolute invisible group-hover/title:visible opacity-0 group-hover/title:opacity-100 bottom-full mb-2 left-1/2 -translate-x-1/2 px-2 py-1 bg-slate-800 rounded text-sm whitespace-nowrap transition-all duration-200 z-10 shadow-lg shadow-black/20'>
                {props.movie.title}
              </span>
              {props.movie.title}
            </h3>

            <p class='text-sm text-blue-200'>{props.movie.year}</p>
          </div>
        </div>
      </div>
    </Link>
  );
}
