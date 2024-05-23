export async function load({ fetch, url }) {
  const res = await fetch(`/api/v1/movie${url.search}`);

  const {
    Items: { Movies, Pages, Page, Count }
  } = await res.json();

  return {
    movies: Movies,
    pages: Pages,
    page: Page,
    count: Count
  };
}
