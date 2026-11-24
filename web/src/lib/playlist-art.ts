/** How many images a playlist collage shows: a 2×2 grid. */
export const PLAYLIST_COLLAGE_SIZE = 4;

/**
 * The images in a playlist's art square, on its card and on the music
 * playlist page (design-system §3.2): the playlist's own cover; else a 2×2
 * collage when four items have artwork; else the first item's image alone,
 * because two or three would leave holes in the grid; else none.
 */
export function playlistArtUrls(coverUrl: string, artworkUrls: readonly string[]) {
  if (coverUrl !== "") return [coverUrl];
  if (artworkUrls.length >= PLAYLIST_COLLAGE_SIZE) {
    return artworkUrls.slice(0, PLAYLIST_COLLAGE_SIZE);
  }
  return artworkUrls.slice(0, 1);
}
