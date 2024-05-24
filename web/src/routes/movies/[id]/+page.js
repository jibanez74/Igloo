export async function load({ fetch, params }) {
  const res = await fetch(`/api/v1/movie/${params.id}`);

  const { Item } = await res.json();

  return Item;
}
