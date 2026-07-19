import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MovieDetailsMetadataChips from "@/components/movies/MovieDetailsMetadataChips";

const baseProps = {
  criticRating: null,
  audienceRating: null,
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

    expect(screen.getByLabelText("Runtime: 1 hr 56 min")).toBeInTheDocument();
  });

  it("uses spoken runtime in the accessible label when minutes are available", () => {
    render(
      <MovieDetailsMetadataChips
        {...baseProps}
        runtime="1 hr 56 min"
        runTimeMins={116}
      />,
    );

    expect(
      screen.getByLabelText("Runtime: 1 hour 56 minutes"),
    ).toBeInTheDocument();
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
      expect(screen.getByLabelText(description)).toBeInTheDocument();
    }
    expect(screen.getByText("4K")).toBeInTheDocument();
    expect(screen.getByText("CC")).toBeInTheDocument();
  });
});
