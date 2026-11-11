import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MusicDetailBackdrop, {
  MusicDetailBackdropSkeleton,
} from "@/components/music/MusicDetailBackdrop";

describe("MusicDetailBackdrop", () => {
  it("is decorative: the whole band is hidden from assistive tech", () => {
    const { container } = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" />,
    );

    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("shows the artwork when there is one", () => {
    const { container } = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" />,
    );

    const image = container.querySelector("img");
    expect(image).toHaveAttribute("src", "/api/static/albums/1.jpg");
    expect(image).toHaveAttribute("alt", "");
    expect(container.querySelector("svg")).toBeNull();
  });

  // The cover block beside the title already shows the "no cover" icon, so
  // the band stays plain: a second icon here read as a glitch.
  it("is a plain band, with no icon, without artwork and after a load error", () => {
    const { container, rerender } = render(<MusicDetailBackdrop imageUrl="" />);
    const band = () => container.firstElementChild?.firstElementChild;

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    expect(band()).toHaveClass("bg-muted");

    rerender(<MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" />);
    const image = container.querySelector("img");
    expect(image).not.toBeNull();
    fireEvent.error(image!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).toBeNull();
    expect(band()).toHaveClass("bg-muted");
  });

  // The app shell pads the page; the band cancels that padding at the sides
  // and at the top, so it starts right under the app header.
  it("reaches the page's side edges and the app header", () => {
    const band = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" />,
    ).container.firstElementChild;
    const skeleton = render(<MusicDetailBackdropSkeleton />).container
      .firstElementChild;

    for (const shell of [band, skeleton]) {
      expect(shell).toHaveClass("-mt-6", "-mx-4", "sm:-mx-6", "lg:-mx-8");
    }
  });

  // The fallback used to stop at the phone heights and stay h-52 above md,
  // while the image and the skeleton followed the clamped 21:9 ratio.
  it("sizes the fallback and the skeleton band exactly like the image", () => {
    const bandClasses = (element: Element | null | undefined) =>
      (element?.className ?? "")
        .split(" ")
        .filter(token => /^(?:\w+:)?(?:h-|w-|aspect-|max-h-|min-h-)/.test(token));

    const image = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" />,
    ).container.querySelector("img");
    const fallback = render(
      <MusicDetailBackdrop imageUrl="" />,
    ).container.firstElementChild?.firstElementChild;
    const skeleton = render(<MusicDetailBackdropSkeleton />).container
      .firstElementChild?.firstElementChild;

    expect(bandClasses(image)).toContain("md:max-h-[min(42vh,22rem)]");
    expect(bandClasses(fallback)).toEqual(bandClasses(image));
    expect(bandClasses(skeleton)).toEqual(bandClasses(image));
  });
});
