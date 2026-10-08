import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import CrewDisclosure from "@/components/shared/CrewDisclosure";

const credits = Array.from({ length: 40 }, (_, i) => ({
  key: `credit-${i}`,
  job: i % 2 ? "Gaffer" : "Set Decorator",
  department: i % 3 ? "Art" : null,
  name: `Crew Member ${i}`,
}));

describe("CrewDisclosure", () => {
  // The list used to be a fixed-height inner scroller; the window is the one
  // scroll container (design-system §3.1), so the crew expands in place.
  it("expands the full crew inline rather than inside a nested scroller", async () => {
    const user = userEvent.setup();
    render(<CrewDisclosure credits={credits} />);

    const toggle = screen.getByRole("button", { name: "Show all crew" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("list")).not.toBeInTheDocument();

    await user.click(toggle);

    const list = screen.getByRole("list", { name: "Full crew list, 40 credits" });
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(toggle).toHaveAttribute("aria-controls", list.id);
    expect(list.className).not.toMatch(/overflow-y-auto|max-h-/);
    expect(list).not.toHaveAttribute("tabindex");
    expect(list.className).toContain("sm:grid-cols-2");
    expect(screen.getAllByRole("listitem")).toHaveLength(40);

    await user.click(screen.getByRole("button", { name: "Show less" }));
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });

  it("renders nothing without credits", () => {
    const { container } = render(<CrewDisclosure credits={[]} />);

    expect(container).toBeEmptyDOMElement();
  });
});
