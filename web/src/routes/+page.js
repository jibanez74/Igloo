export async function load({ fetch }) {
  const res = await fetch('/api/v1/recent');

  const r = await res.json();

  return {
    movies: r.Items.Movies
  };
}
