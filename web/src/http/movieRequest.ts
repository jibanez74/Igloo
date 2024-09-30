import type { JsonResponse } from "../types/JsonResponse";
import type { SimpleMovie } from "../types/Movie";

export const getLatestMovies = async (): Promise<
  JsonResponse<SimpleMovie[]>
> => {
  try {
    const res = await fetch("/api/v1/movie/latest");
    return res.json();
  } catch (err) {
    throw new Error(err.message);
  }
};
