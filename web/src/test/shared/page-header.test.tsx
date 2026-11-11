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

  it("puts a library's counts and controls on the title's row, outside the heading", () => {
    render(
      <PageHeader
        icon={Film}
        title="Movie Library"
        meta={<p>63 Movies</p>}
        actions={<button type="button">More options</button>}
      />,
    );

    const heading = screen.getByRole("heading", { level: 1, name: "Movie Library" });
    const row = heading.parentElement;
    expect(row).toHaveClass("flex", "flex-wrap", "items-center");
    expect(row).toContainElement(screen.getByText("63 Movies"));
    // The controls keep to the end of the row, also when it wraps.
    const actions = screen.getByRole("button", { name: "More options" });
    expect(row).toContainElement(actions);
    expect(actions.parentElement).toHaveClass("ml-auto");
    // Neither is part of the page's name.
    expect(heading).not.toContainElement(screen.getByText("63 Movies"));
    expect(heading).not.toContainElement(actions);
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
