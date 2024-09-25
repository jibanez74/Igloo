import type { Component } from "solid-js";
import type { SimpleMovie } from "../types/Movie";

type MovieCardProps = {
  movie: SimpleMovie;
};

const MovieCard: Component<MovieCardProps> = props => (
  <div class='bg-primary hover:bg-secondary rounded-lg shadow-lg overflow-hidden transition-colors duration-300'>
    <a href={`/movies/details/${props.movie.ID}`}>
      <img
        src={props.movie.thumb}
        alt={props.movie.title}
        class='w-full h-64 object-cover'
        loading='lazy'
      />
    </a>

    <div class='p-4'>
      <h3 class='text-light text-xl font-bold mb-2'>{props.movie.title}</h3>

      <p class='text-blue-300'>{props.movie.year}</p>
    </div>
  </div>
);

export default MovieCard;
