export const getMovieByID = async id => {
  try {
    const res = await fetch(`/api/v1/movie/by-id/${id}`);

    const r = await res.json();

    if (r.Error) {
      throw new Error(r.message);
    }

    return r.data;
  } catch (err) {
    throw new Error(`Unable to process request`);
  }
};

export const getMoviesWithPagination = async (page, keyword) => {};
