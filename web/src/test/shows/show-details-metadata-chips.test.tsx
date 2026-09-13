import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ShowDetailsMetadataChips from "@/components/shows/ShowDetailsMetadataChips";

const baseProps = {
  tmdbVoteAverage: null,
  certificationLabel: null,
  status: null,
  firstAirDate: null,
  lastAirDate: null,
  seasonCount: 1,
  tmdbSeasonCount: null,
  availableEpisodeCount: 1,
  tmdbEpisodeCount: null,
};

describe("ShowDetailsMetadataChips", () => {
  it("states availability in words when a season is only partly present", () => {
    render(
      <ShowDetailsMetadataChips
        {...baseProps}
        seasonCount={2}
        tmdbSeasonCount={5}
        availableEpisodeCount={3}
        tmdbEpisodeCount={40}
      />,
    );

    // Availability is never conveyed by color alone.
    expect(
      screen.getByText("2 of 5 seasons available in this library"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("3 of 40 episodes available in this library"),
    ).toBeInTheDocument();
  });

  it("drops the 'of' form when everything TMDB knows about is present", () => {
    render(
      <ShowDetailsMetadataChips
        {...baseProps}
        seasonCount={2}
        tmdbSeasonCount={2}
        availableEpisodeCount={20}
        tmdbEpisodeCount={20}
      />,
    );

    expect(screen.getByText("2 seasons in this library")).toBeInTheDocument();
    expect(screen.getByText("20 episodes in this library")).toBeInTheDocument();
  });

  it("singularizes a lone season and episode", () => {
    render(
      <ShowDetailsMetadataChips
        {...baseProps}
        seasonCount={1}
        availableEpisodeCount={1}
      />,
    );

    expect(screen.getByText("1 season in this library")).toBeInTheDocument();
    expect(screen.getByText("1 episode in this library")).toBeInTheDocument();
  });

  it("renders an air-date range and collapses it when both years match", () => {
    const { unmount } = render(
      <ShowDetailsMetadataChips
        {...baseProps}
        firstAirDate="2024-03-01"
        lastAirDate="2026-05-20"
      />,
    );

    expect(screen.getByText("Aired 2024 to 2026")).toBeInTheDocument();
    unmount();

    render(
      <ShowDetailsMetadataChips
        {...baseProps}
        firstAirDate="2024-03-01"
        lastAirDate="2024-11-02"
      />,
    );

    expect(screen.getByText("Aired 2024")).toBeInTheDocument();
  });

  it("speaks the certification and series status as list item content", () => {
    render(
      <ShowDetailsMetadataChips
        {...baseProps}
        certificationLabel="TV-14"
        status="Returning Series"
      />,
    );

    // The chip recipe: the spoken value is content, never an aria-label on the
    // list item, whose implicit role does not support one.
    expect(screen.getByText("Rated TV-14")).toBeInTheDocument();
    expect(
      screen.getByText("Series status: Returning Series"),
    ).toBeInTheDocument();

    for (const item of screen.getAllByRole("listitem")) {
      expect(item).not.toHaveAttribute("aria-label");
    }
  });

  it("renders the TMDB score as the labelled badge, not a tiered rating chip", () => {
    render(<ShowDetailsMetadataChips {...baseProps} tmdbVoteAverage={8.4} />);

    expect(
      screen.getByText("TMDB user score: 8.4 out of 10"),
    ).toBeInTheDocument();
    expect(screen.getByText("TMDB")).toBeInTheDocument();
    // The critic/audience chips are the only tiered ones; a different metric
    // must not borrow their look.
    expect(document.querySelector(".bg-aurora")).toBeNull();
  });

  it("omits the TMDB score when there is none", () => {
    render(<ShowDetailsMetadataChips {...baseProps} tmdbVoteAverage={0} />);

    expect(screen.queryByText(/TMDB user score/)).not.toBeInTheDocument();
  });
});
