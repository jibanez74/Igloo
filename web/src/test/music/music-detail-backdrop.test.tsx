import { fireEvent, render, screen } from "@testing-library/react";
import { Disc3 } from "lucide-react";
import { describe, expect, it } from "vitest";
import MusicDetailBackdrop from "@/components/music/MusicDetailBackdrop";

describe("MusicDetailBackdrop", () => {
  it("is decorative: the whole band is hidden from assistive tech", () => {
    const { container } = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" fallbackIcon={Disc3} />,
    );

    expect(container.firstElementChild).toHaveAttribute("aria-hidden", "true");
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("shows the artwork when there is one", () => {
    const { container } = render(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" fallbackIcon={Disc3} />,
    );

    const image = container.querySelector("img");
    expect(image).toHaveAttribute("src", "/api/static/albums/1.jpg");
    expect(image).toHaveAttribute("alt", "");
    expect(container.querySelector("svg")).toBeNull();
  });

  it("falls back to the icon without artwork and after a load error", () => {
    const { container, rerender } = render(
      <MusicDetailBackdrop imageUrl="" fallbackIcon={Disc3} />,
    );

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();

    rerender(
      <MusicDetailBackdrop imageUrl="/api/static/albums/1.jpg" fallbackIcon={Disc3} />,
    );
    const image = container.querySelector("img");
    expect(image).not.toBeNull();
    fireEvent.error(image!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg")).not.toBeNull();
  });
});
