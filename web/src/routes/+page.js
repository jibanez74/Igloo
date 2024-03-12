export async function load({ fetch }) {
  const res = await fetch("http://localhost:8080/api/v1/movies/recently-added");
}
