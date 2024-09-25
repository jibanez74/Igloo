export async function getLatestMovies() {
  return fetch("/api/v1/movie/latest");
}
