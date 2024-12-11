import { LoaderFunctionArgs } from "react-router-dom";
import api from "../lib/api";
import getError from "../lib/getError";
import type { Movie } from "../types/Movie";

type MovieResponse = {
  movie: Movie;
};

export default async function loader({ params }: LoaderFunctionArgs) {
  try {
    const { data } = await api.get<MovieResponse>(`/auth/movie/${params.id}`);

    if (!data.movie) {
      throw new Error("no movie was returned");
    }

    return data.movie;
  } catch (err) {
    throw getError(err);
  }
}
