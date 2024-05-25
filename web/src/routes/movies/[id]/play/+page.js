export async function load({ params }) {
  return {
    filePath: `/api/v1/playback/direct/${params.id}`
  };
}
