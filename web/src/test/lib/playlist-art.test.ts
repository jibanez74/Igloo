import { describe, expect, it } from "vitest";
import { playlistArtUrls } from "@/lib/playlist-art";

describe("playlistArtUrls", () => {
  it.each([
    ["the playlist's own cover wins", "/own.jpg", ["/a", "/b", "/c", "/d"], ["/own.jpg"]],
    [
      "four or more items make a 2×2 collage",
      "",
      ["/a", "/b", "/c", "/d", "/e"],
      ["/a", "/b", "/c", "/d"],
    ],
    ["one to three items show the first alone", "", ["/a", "/b", "/c"], ["/a"]],
    ["nothing leaves the placeholder", "", [], []],
  ])("%s", (_case, cover, artwork, want) => {
    expect(playlistArtUrls(cover, artwork)).toEqual(want);
  });
});
