import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import BrandMark from "@/components/app/BrandMark";
import { BRAND_MARK_GLYPH_VIEWBOX, BRAND_MARK_PATH } from "@/lib/brand-mark";

describe("BrandMark", () => {
  it("draws the shared igloo glyph as a decorative, currentColor svg", () => {
    const { container } = render(<BrandMark className="size-6 text-primary" />);
    const svg = container.querySelector("svg");

    expect(svg).not.toBeNull();
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg).toHaveAttribute("viewBox", BRAND_MARK_GLYPH_VIEWBOX);
    expect(svg).toHaveClass("shrink-0", "size-6", "text-primary");

    const path = svg?.querySelector("path");
    expect(path).toHaveAttribute("d", BRAND_MARK_PATH);
    expect(path).toHaveAttribute("fill", "currentColor");
    expect(path).toHaveAttribute("fill-rule", "evenodd");
  });
});
