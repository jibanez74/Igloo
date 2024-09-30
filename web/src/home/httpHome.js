export const getLatestMovies = async () => {
  const res = await fetch("/api/v1/movie/latest");

  const r = await res.json();

  if (!res.ok) {
    throw new Error(`${res.status} - ${r.Message}`);
  }

  return r.data;
};
