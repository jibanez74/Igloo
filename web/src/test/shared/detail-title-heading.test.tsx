import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import DetailTitleHeading from "@/components/shared/DetailTitleHeading";

describe("DetailTitleHeading", () => {
  // The title and year are flex items, so nothing visible separates them; the
  // text node between them keeps a screen reader from saying "Gladiator(2000)".
  it("separates the title from the year in the heading's text", () => {
    render(
      <DetailTitleHeading
        id="movie-title"
        title="Gladiator"
        year={2000}
        dateTime="2000-05-05"
      />,
    );

    const heading = screen.getByRole("heading", { level: 1 });
    expect(heading.textContent).toBe("Gladiator (2000)");
    expect(heading).toHaveAccessibleName("Gladiator (2000)");
  });

  it("renders only the title when the catalog has no year", () => {
    render(
      <DetailTitleHeading id="movie-title" title="Gladiator" year={null} dateTime={null} />,
    );

    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Gladiator");
  });
});
