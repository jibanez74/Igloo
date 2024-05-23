export async function load({ params }) {
  return {
    url: `/api/v1/playback/direct/${params.id}`
  };
}
