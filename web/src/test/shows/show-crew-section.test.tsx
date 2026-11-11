import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import ShowCrewSection from "@/components/shows/ShowCrewSection";
import type { ShowCrewCreditType, ShowPersonType } from "@/types";

// PersonLink needs a router; these tests are about the heading and the rows.
vi.mock("@/components/shared/PersonLink", () => ({
  default: ({ children }: { children: ReactNode }) => <span>{children}</span>,
}));

const creator: ShowPersonType = {
  id: 1,
  name: "Rowan Hale",
  profile: { String: "", Valid: false },
  tmdb_id: 501,
};

const gaffer: ShowCrewCreditType = {
  credit_id: "c-1",
  artist_id: 7,
  department: "Lighting",
  job: "Gaffer",
  episode_count: 8,
  artist_name: "Quinn Marsh",
  artist_profile: { String: "", Valid: false },
  tmdb_id: 502,
};

describe("ShowCrewSection", () => {
  it("is headed Key Crew when the show bills its creators", () => {
    render(<ShowCrewSection creators={[creator]} crew={[gaffer]} />);

    const heading = screen.getByRole("heading", { name: "Key Crew" });
    expect(heading).toHaveAttribute("id", "crew-heading");
    expect(screen.getByText("Rowan Hale")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Show all crew" }),
    ).toBeInTheDocument();
  });

  // The heading used to promise key crew over a section that held nothing
  // but the "Show all crew" button.
  it("is headed Crew when there are no creators, only the full crew", () => {
    render(<ShowCrewSection creators={[]} crew={[gaffer]} />);

    const heading = screen.getByRole("heading", { name: "Crew" });
    expect(heading).toHaveAttribute("id", "crew-heading");
    expect(
      screen.queryByRole("heading", { name: "Key Crew" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Creator")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Show all crew" }),
    ).toBeInTheDocument();
  });

  it("renders nothing without creators or crew", () => {
    const { container } = render(<ShowCrewSection creators={[]} crew={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
