import api from "../utils/api";

export const getMovieByID = async id => {
  try {
    const res = await api.get(`/movie/${id}`);

    return res.data.movie;
  } catch (err) {
    console.error(err);
    throw err;
  }
};

export const getMoviesWithPagination = async (page, keyword) => {
  try {
    const res = await api.get("/movie", {
      params: {
        keyword,
        page,
      },
    });

    return res.data;
  } catch (err) {
    console.error(err);
    throw err;
  }
};
