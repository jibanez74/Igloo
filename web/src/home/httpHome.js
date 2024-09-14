import api from "../utils/api";

export const getLatestMovies = async () => {
  try {
    const res = await api.get("/movie/latest");

    return res.data.movies;
  } catch (err) {
    console.error(err);
  }
};
