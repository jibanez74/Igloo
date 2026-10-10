import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import MovieAboutSection from "@/components/movies/MovieAboutSection";
import ShowAboutSection from "@/components/shows/ShowAboutSection";

describe("MovieAboutSection", () => {
  it("names the original language instead of printing its code", () => {
    render(
      <MovieAboutSection
        movieTitle="Gladiator"
        status="Released"
        language="en"
        budget={null}
        revenue={null}
        companies={[]}
      />,
    );

    expect(screen.getByText("English")).toBeInTheDocument();
    expect(screen.queryByText("EN")).not.toBeInTheDocument();
  });

  // An unmatched movie (no TMDB data) has nothing for this section to say.
  it("renders nothing when it has no rows", () => {
    const { container } = render(
      <MovieAboutSection
        movieTitle="Kikis Delivery Service 1989 BluRay 1080p"
        status={null}
        language={null}
        budget={0}
        revenue={null}
        companies={[]}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});

describe("ShowAboutSection", () => {
  it("renders nothing when it has no rows", () => {
    const { container } = render(
      <ShowAboutSection
        name="Untitled Show"
        originalName="Untitled Show"
        status={null}
        type={null}
        language={null}
        firstAirDate={null}
        lastAirDate={null}
        networks={[]}
        companies={[]}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});
