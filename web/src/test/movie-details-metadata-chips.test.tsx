import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MovieDetailsMetadataChips from "@/components/MovieDetailsMetadataChips";

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
});
