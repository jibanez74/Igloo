import { render, screen } from "@testing-library/react";
import { Film } from "lucide-react";
import { describe, expect, it } from "vitest";
import PageHeader from "@/components/shared/PageHeader";

describe("PageHeader", () => {
  it("holds the page's h1, named by the title alone", () => {
    const { container } = render(
      <PageHeader
        icon={Film}
        title="Movie Library"
        description="Browse, organize, and enjoy your film collection"
      />,
    );

    expect(
      screen.getByRole("heading", { level: 1, name: "Movie Library" }),
    ).toBeInTheDocument();
    expect(container.querySelectorAll("h1")).toHaveLength(1);
    // The icon is decoration: it must not reach the heading's name.
    expect(container.querySelector("h1 svg")).toHaveAttribute("aria-hidden", "true");
    expect(
      screen.getByText("Browse, organize, and enjoy your film collection"),
    ).toBeInTheDocument();
  });

  it("draws no description line when there is none", () => {
    const { container } = render(<PageHeader icon={Film} title="Search" />);

    expect(container.querySelector("header p")).toBeNull();
  });

  it("takes markup as its title", () => {
    render(
      <PageHeader
        icon={Film}
        title={
          <>
            Search results for <span className="text-primary">‘dune’</span>
          </>
        }
      />,
    );

    expect(
      screen.getByRole("heading", { level: 1, name: "Search results for ‘dune’" }),
    ).toBeInTheDocument();
  });

  it("enters with the section motion, and holds still under reduced motion", () => {
    const { container } = render(<PageHeader icon={Film} title="Movie Library" />);

    expect(container.querySelector("header")).toHaveClass(
      "animate-in",
      "fade-in-0",
      "motion-reduce:animate-none",
    );
  });
});
