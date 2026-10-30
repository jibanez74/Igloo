import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import DetailBackdrop from "@/components/shared/DetailBackdrop";
import { MEDIA_FALLBACK_SURFACE_CLASS } from "@/lib/constants";

describe("DetailBackdrop", () => {
  it("shows the backdrop image under the scrims when there is one", () => {
    const { container } = render(
      <DetailBackdrop backdropUrl="/api/tmdb/images/w1280/backdrop.jpg" />,
    );

    expect(container.querySelector("img")).toHaveAttribute(
      "src",
      "/api/tmdb/images/w1280/backdrop.jpg",
    );
    expect(container.querySelector(".bg-media-fallback")).toBeNull();
  });

  it("shows the poster blurred over the tile when the title has no backdrop (design-system §1.2)", () => {
    const { container } = render(
      <DetailBackdrop
        backdropUrl={null}
        posterUrl="/api/tmdb/images/w500/poster.jpg"
      />,
    );

    const poster = container.querySelector("img");
    expect(poster).toHaveAttribute("src", "/api/tmdb/images/w500/poster.jpg");
    expect(poster?.className).toMatch(/\bblur-/);
    // The navy tile stays underneath so a transparent or failed poster never
    // exposes a themed surface to the literal scrims.
    expect(container.querySelector(".bg-media-fallback")).not.toBeNull();
    expect(
      container.querySelectorAll("[class*='bg-linear-to-']").length,
    ).toBeGreaterThanOrEqual(2);
  });

  it("drops a poster that fails to load and keeps the tile", () => {
    const { container } = render(
      <DetailBackdrop backdropUrl={null} posterUrl="/broken.jpg" />,
    );

    fireEvent.error(container.querySelector("img")!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector(".bg-media-fallback")).not.toBeNull();
  });

  it("falls back to the fixed media tile, never the themed muted surface (design-system §1.2)", () => {
    const { container } = render(<DetailBackdrop backdropUrl={null} />);

    expect(container.querySelector("img")).toBeNull();
    const tile = container.querySelector(".bg-media-fallback");
    expect(tile).not.toBeNull();
    for (const cls of MEDIA_FALLBACK_SURFACE_CLASS.split(" ")) {
      expect(tile).toHaveClass(cls);
    }
    // The near-white muted surface is what turned the scrims into a smear in light.
    expect(container.querySelector(".bg-muted")).toBeNull();
    // The literal scrims still sit on top of the tile.
    expect(
      container.querySelectorAll("[class*='bg-linear-to-']").length,
    ).toBeGreaterThanOrEqual(2);
  });
});
