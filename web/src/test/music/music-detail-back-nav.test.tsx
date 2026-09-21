import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MusicDetailBackNav from "@/components/music/MusicDetailBackNav";
import { linkSearch } from "../helpers/router-link-mock";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

describe("MusicDetailBackNav", () => {
  it("is a page-navigation landmark linking back to the owning library tab", () => {
    render(<MusicDetailBackNav tab="musicians" label="Back to Musicians" />);

    const nav = screen.getByRole("navigation", { name: "Page navigation" });
    const link = screen.getByRole("link", {
      name: "Back to Musicians library",
    });

    expect(nav).toContainElement(link);
    expect(link).toHaveAttribute("href", "/music");
    expect(linkSearch(link)).toEqual({ tab: "musicians" });
    expect(link).toHaveTextContent("Back to Musicians");
  });

  it("takes an outer class for the page's spacing", () => {
    render(
      <MusicDetailBackNav
        tab="playlists"
        label="Back to Playlists"
        className="mt-8"
      />,
    );

    expect(screen.getByRole("navigation")).toHaveClass("mt-8");
  });
});
