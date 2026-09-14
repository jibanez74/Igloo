import { screen } from "@testing-library/react";
import { Film } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import PosterCard from "@/components/shared/PosterCard";
import { renderWithQueryClient } from "../helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

// `as const` keeps `to` a route literal rather than widening it to string,
// which is what LinkProps checks against the generated route tree.
const baseProps = {
  detailsLink: { to: "/movies/$id", params: { id: "7" } } as const,
  posterUrl: "https://image.tmdb.org/t/p/w500/poster.jpg",
  fallbackIcon: Film,
  title: "Ember Line",
  detailsLabel: "Ember Line 2024",
};

function progressFill(container: HTMLElement) {
  const track = container.querySelector<HTMLElement>("[aria-hidden='true'].h-1");
  return track?.firstElementChild as HTMLElement | undefined;
}

describe("PosterCard", () => {
  it("links the poster to the details page and names it for assistive tech", () => {
    renderWithQueryClient(<PosterCard {...baseProps} />);

    expect(
      screen.getByRole("link", { name: "Ember Line 2024" }),
    ).toHaveAttribute("href", "/movies/7");
    expect(
      screen.getByRole("heading", { name: "Ember Line" }),
    ).toBeInTheDocument();
  });

  it("renders no play control and no hover wash for a card with nothing to play", () => {
    const { container } = renderWithQueryClient(<PosterCard {...baseProps} />);

    // The details link is the only link: a show or an unreleased title has no
    // single thing to play, so the overlay would wash out for nothing.
    expect(screen.getAllByRole("link")).toHaveLength(1);
    expect(container.querySelector(".bg-black\\/30")).toBeNull();
  });

  it("reveals a play action, and its wash, when there is one thing to play", () => {
    const { container } = renderWithQueryClient(
      <PosterCard
        {...baseProps}
        playLink={{ to: "/movies/$id/play", params: { id: "7" } } as const}
        playLabel="Play Ember Line 2024"
      />,
    );

    expect(
      screen.getByRole("link", { name: "Play Ember Line 2024" }),
    ).toHaveAttribute("href", "/movies/7/play");
    expect(container.querySelector(".bg-black\\/30")).not.toBeNull();
  });

  it("fills the progress bar in proportion to the saved position", () => {
    const { container } = renderWithQueryClient(
      <PosterCard
        {...baseProps}
        watchProgress={{ progressSec: 1350, durationSec: 5400 }}
      />,
    );

    expect(progressFill(container)?.style.width).toBe("25%");
  });

  it("omits the progress bar when there is no progress or no duration", () => {
    const { container: bare } = renderWithQueryClient(
      <PosterCard {...baseProps} />,
    );
    expect(progressFill(bare)).toBeUndefined();

    const { container: unknownDuration } = renderWithQueryClient(
      <PosterCard
        {...baseProps}
        watchProgress={{ progressSec: 600, durationSec: 0 }}
      />,
    );
    expect(progressFill(unknownDuration)).toBeUndefined();
  });

  it("omits the subtitle line when the caller has no second line", () => {
    const { rerender } = renderWithQueryClient(<PosterCard {...baseProps} />);
    expect(screen.queryByText("2024")).not.toBeInTheDocument();

    rerender(<PosterCard {...baseProps} subtitle="2024" />);
    expect(screen.getByText("2024")).toBeInTheDocument();
  });

  it("falls back to the icon when the poster fails to load", () => {
    renderWithQueryClient(<PosterCard {...baseProps} posterUrl="" />);

    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });
});
