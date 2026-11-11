import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import MovieKeyCrewSection from "@/components/movies/MovieKeyCrewSection";
import { crewSectionLabels, movieHasKeyCrew } from "@/lib/crew";
import type { LibraryMovieCrewType } from "@/types/movies";

// PersonLink needs a router; these tests are about the heading and the rows.
vi.mock("@/components/shared/PersonLink", () => ({
  default: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}));

const credit = (
  id: number,
  job: string,
  department: string,
  artist_name: string,
): LibraryMovieCrewType => ({ id, job, department, artist_name, tmdb_id: 900 + id });

const director = credit(1, "Director", "Directing", "Jordan Lee");
const writer = credit(2, "Screenplay", "Writing", "Casey North");
const editor = credit(3, "Editor", "Editing", "Sam Ortiz");

describe("MovieKeyCrewSection", () => {
  it("is headed Key Crew when there is a director or a writer", () => {
    render(<MovieKeyCrewSection crew={[director, writer, editor]} />);

    const heading = screen.getByRole("heading", { name: "Key Crew" });
    expect(heading).toHaveAttribute("id", "crew-heading");
    expect(screen.getByText("Jordan Lee")).toBeInTheDocument();
    expect(screen.getByText("Casey North")).toBeInTheDocument();
  });

  // The heading used to promise key crew over a section that held nothing
  // but the "Show all crew" button.
  it("is headed Crew when the crew has no director and no writer", () => {
    render(<MovieKeyCrewSection crew={[editor]} />);

    const heading = screen.getByRole("heading", { name: "Crew" });
    expect(heading).toHaveAttribute("id", "crew-heading");
    expect(
      screen.queryByRole("heading", { name: "Key Crew" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Show all crew" }),
    ).toBeInTheDocument();
  });

  it("renders nothing without crew", () => {
    const { container } = render(<MovieKeyCrewSection crew={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});

describe("crew section labels", () => {
  it("finds key crew from a director or a writer alone", () => {
    expect(movieHasKeyCrew([director])).toBe(true);
    expect(movieHasKeyCrew([writer])).toBe(true);
    expect(movieHasKeyCrew([editor])).toBe(false);
    expect(movieHasKeyCrew([])).toBe(false);
  });

  // The skip link and the heading it jumps to must read the same word.
  it("names the heading and its skip link alike", () => {
    expect(crewSectionLabels(true)).toEqual({
      heading: "Key Crew",
      skipLabel: "Skip to key crew",
    });
    expect(crewSectionLabels(false)).toEqual({
      heading: "Crew",
      skipLabel: "Skip to crew",
    });
  });
});
