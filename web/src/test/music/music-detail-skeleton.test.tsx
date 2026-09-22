import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MusicDetailSkeleton from "@/components/music/MusicDetailSkeleton";

describe("MusicDetailSkeleton", () => {
  it("is a single labelled status region per variant", () => {
    const { unmount } = render(<MusicDetailSkeleton variant="album" />);
    expect(
      screen.getByRole("status", { name: "Loading album details" }),
    ).toBeInTheDocument();
    unmount();

    render(<MusicDetailSkeleton variant="musician" />);
    expect(
      screen.getByRole("status", { name: "Loading musician details" }),
    ).toBeInTheDocument();
  });

  it("mirrors the album hero: a square cover, no discography grid", () => {
    const { container } = render(<MusicDetailSkeleton variant="album" />);

    expect(container.querySelector(".rounded-xl.w-44")).not.toBeNull();
    expect(container.querySelector(".rounded-full.w-48")).toBeNull();
    expect(container.querySelectorAll(".grid")).toHaveLength(0);
    // Play, Shuffle and the round admin menu button.
    expect(container.querySelectorAll(".h-12, .size-12")).toHaveLength(3);
  });

  it("mirrors the playlist header: a square cover with no backdrop band and no overlap", () => {
    const { container } = render(<MusicDetailSkeleton variant="playlist" />);

    expect(
      screen.getByRole("status", { name: "Loading playlist details" }),
    ).toBeInTheDocument();
    expect(container.querySelector(".rounded-xl.w-40")).not.toBeNull();
    expect(container.querySelector(".aspect-21\\/9")).toBeNull();
    expect(container.querySelector(".-mt-20")).toBeNull();
    // Play and Shuffle only: the playlist page has no round menu button.
    expect(container.querySelectorAll(".h-12, .size-12")).toHaveLength(2);
    expect(container.querySelectorAll(".h-14")).toHaveLength(8);
  });

  it("mirrors the musician hero: a round thumb and the six-card discography", () => {
    const { container } = render(<MusicDetailSkeleton variant="musician" />);

    expect(container.querySelector(".rounded-full.w-48")).not.toBeNull();
    expect(container.querySelector(".rounded-xl.w-44")).toBeNull();
    expect(container.querySelectorAll(".grid > div")).toHaveLength(6);
    expect(container.querySelectorAll(".h-12")).toHaveLength(2);
  });

  it("hides its visuals under the status label", () => {
    const { container } = render(<MusicDetailSkeleton variant="album" />);

    const status = screen.getByRole("status");
    const visualChildren = Array.from(status.children).filter(
      child => !child.classList.contains("sr-only"),
    );
    expect(visualChildren.length).toBeGreaterThan(0);
    for (const child of visualChildren) {
      expect(child).toHaveAttribute("aria-hidden", "true");
    }
    expect(container.querySelectorAll(".h-14")).toHaveLength(8);
  });
});
