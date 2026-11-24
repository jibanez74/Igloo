import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MusicDetailArt, {
  MusicDetailArtSkeleton,
} from "@/components/music/MusicDetailArt";

describe("MusicDetailArt", () => {
  it.each([
    ["album", "Album cover for Glacier Sessions"],
    ["musician", "Photo of Glacier Sessions"],
    ["playlist", "Playlist cover for Glacier Sessions"],
  ] as const)("names the %s artwork after its subject", (variant, alt) => {
    render(
      <MusicDetailArt
        variant={variant}
        src="/api/static/art/1.jpg"
        name="Glacier Sessions"
      />,
    );

    expect(screen.getByRole("img", { name: alt })).toHaveAttribute(
      "src",
      "/api/static/art/1.jpg",
    );
  });

  // The hero art is the page's LCP image: eager, unlike the card recipe.
  it("loads eagerly with intrinsic dimensions", () => {
    render(
      <MusicDetailArt variant="album" src="/api/static/art/1.jpg" name="A" />,
    );

    const image = screen.getByRole("img");
    expect(image).not.toHaveAttribute("loading");
    expect(image).not.toHaveAttribute("fetchpriority");
    expect(image).toHaveAttribute("width", "640");
    expect(image).toHaveAttribute("height", "640");
  });

  it("swaps missing or broken artwork for a labelled placeholder", () => {
    const { rerender } = render(
      <MusicDetailArt variant="playlist" src="" name="Late Shift" />,
    );
    expect(
      screen.getByRole("img", { name: "No cover available" }),
    ).toBeInTheDocument();

    rerender(
      <MusicDetailArt
        variant="musician"
        src="/api/static/art/2.jpg"
        name="Aurora Pines"
      />,
    );
    fireEvent.error(screen.getByRole("img", { name: "Photo of Aurora Pines" }));

    expect(
      screen.getByRole("img", { name: "No image available" }),
    ).toBeInTheDocument();
  });

  const COLLAGE = ["/a.jpg", "/b.jpg", "/c.jpg", "/d.jpg"];

  it("draws four images as one named 2×2 collage in place of src", () => {
    render(
      <MusicDetailArt
        variant="playlist"
        src=""
        collage={COLLAGE}
        name="Late Shift"
      />,
    );

    const collage = screen.getByRole("img", { name: "Playlist cover for Late Shift" });
    expect(collage).toHaveClass("grid-cols-2", "grid-rows-2");
    const cells = Array.from(collage.querySelectorAll("img"));
    expect(cells.map(cell => cell.getAttribute("src"))).toEqual(COLLAGE);
    // The cells are decorative: the grid carries the name, and like the
    // single hero image they load eagerly.
    for (const cell of cells) {
      expect(cell).toHaveAttribute("alt", "");
      expect(cell).not.toHaveAttribute("loading");
    }
  });

  it("swaps the whole collage for the placeholder when one cell fails", () => {
    render(
      <MusicDetailArt
        variant="playlist"
        src=""
        collage={COLLAGE}
        name="Late Shift"
      />,
    );

    const cells = screen
      .getByRole("img", { name: "Playlist cover for Late Shift" })
      .querySelectorAll("img");
    fireEvent.error(cells[2]);

    expect(
      screen.getByRole("img", { name: "No cover available" }),
    ).toBeInTheDocument();
    expect(document.querySelectorAll("img")).toHaveLength(0);
  });

  it("ignores a collage without four images and shows src", () => {
    render(
      <MusicDetailArt
        variant="playlist"
        src="/a.jpg"
        collage={["/a.jpg", "/b.jpg"]}
        name="Late Shift"
      />,
    );

    expect(
      screen.getByRole("img", { name: "Playlist cover for Late Shift" }),
    ).toHaveAttribute("src", "/a.jpg");
  });

  it.each(["album", "musician", "playlist"] as const)(
    "gives the %s skeleton the same box as the art",
    variant => {
      const art = render(
        <MusicDetailArt variant={variant} src="" name="A" />,
      ).container.querySelector("figure > div");
      const skeleton = render(<MusicDetailArtSkeleton variant={variant} />)
        .container.firstElementChild?.firstElementChild;

      const boxClasses = (element: Element | null | undefined) =>
        (element?.className ?? "")
          .split(" ")
          .filter(token => /^(?:\w+:)?(?:w-|rounded-|aspect-)/.test(token));

      expect(boxClasses(skeleton)).toEqual(boxClasses(art));
    },
  );
});
