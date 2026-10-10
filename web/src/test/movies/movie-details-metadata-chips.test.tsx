import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MovieDetailsMetadataChips from "@/components/movies/MovieDetailsMetadataChips";

const baseProps = {
  certificationLabel: null,
  releaseDateStr: null,
  tmdbVoteAverage: null,
};

describe("MovieDetailsMetadataChips", () => {
  it("labels runtime for assistive tech when runtime minutes are missing", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime="1 hr 56 min"
        runTimeMins={null}
      />,
    );

    expect(screen.getByText("Runtime: 1 hr 56 min")).toBeInTheDocument();
  });

  it("uses spoken runtime in the accessible label when minutes are available", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime="1 hr 56 min"
        runTimeMins={116}
      />,
    );

    expect(screen.getByText("Runtime: 1 hour 56 minutes")).toBeInTheDocument();
  });

  it("exposes the certification to assistive tech as list item content", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        certificationLabel="PG-13"
        runtime={null}
        runTimeMins={null}
      />,
    );

    expect(screen.getByText("Rated PG-13")).toBeInTheDocument();
    expect(screen.getByText("PG-13")).toBeInTheDocument();
  });

  it("renders the TMDB score as the labelled badge, never a tiered chip", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime={null}
        runTimeMins={null}
        tmdbVoteAverage={8.2}
      />,
    );

    expect(screen.getByText("TMDB user score: 8.2 out of 10")).toBeInTheDocument();
    expect(screen.getByText("TMDB")).toBeInTheDocument();
    expect(document.querySelector(".bg-aurora")).toBeNull();
    expect(document.querySelector("svg.lucide-star")).toBeNull();
  });

  it("omits the score when there is none", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime={null}
        runTimeMins={null}
        tmdbVoteAverage={0}
      />,
    );

    expect(screen.queryByText(/TMDB user score/)).not.toBeInTheDocument();
  });

  it("renders capability badges with accessible descriptions", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime={null}
        runTimeMins={null}
        capabilityBadges={[
          { label: "4K", description: "4K Ultra HD video" },
          { label: "HDR", description: "High dynamic range video" },
          { label: "7.1", description: "7.1 surround sound audio" },
          { label: "CC", description: "Subtitles available" },
        ]}
      />,
    );

    for (const description of [
      "4K Ultra HD video",
      "High dynamic range video",
      "7.1 surround sound audio",
      "Subtitles available",
    ]) {
      expect(screen.getByText(description)).toBeInTheDocument();
    }
    expect(screen.getByText("4K")).toBeInTheDocument();
    expect(screen.getByText("CC")).toBeInTheDocument();
  });
});
