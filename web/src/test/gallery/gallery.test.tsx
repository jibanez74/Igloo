import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Gallery from "@/components/gallery/Gallery";

// The gallery is what the screenshot baselines shoot (e2e/visual), and that
// run is a CI job, not part of `make check`. These keep a gallery that no
// longer renders, or a section the spec expects, from getting that far.
describe("Gallery", () => {
  it("renders a section for every primitive family the baselines shoot", () => {
    const { container } = render(<Gallery />);

    expect(
      screen.getByRole("heading", { level: 1, name: "Component gallery" }),
    ).toBeInTheDocument();

    const sections = Array.from(
      container.querySelectorAll<HTMLElement>("[data-gallery]"),
      section => section.dataset.gallery,
    );
    expect(sections).toEqual([
      "buttons",
      "badges",
      "fields",
      "surfaces",
      "navigation",
    ]);
  });

  it("names every icon-only button", () => {
    render(<Gallery />);

    for (const button of screen.getAllByRole("button")) {
      expect(button).toHaveAccessibleName();
    }
  });

  it.each([
    ["dialog", "dialog", "Add User"],
    ["alert-dialog", "alertdialog", "Delete playlist"],
    ["dropdown", "menu", "More options"],
    ["popover", "dialog", undefined],
  ] as const)("shows the %s overlay open on its own", (name, role, label) => {
    const { container } = render(<Gallery open={name} />);

    expect(container.querySelector("[data-gallery]")).toBeNull();
    expect(
      container.querySelector(`[data-gallery-overlay="${name}"]`),
    ).not.toBeNull();
    expect(
      screen.getByRole(role, label ? { name: label } : undefined),
    ).toBeInTheDocument();
  });

  it("shows the select overlay with its options listed", () => {
    render(<Gallery open="select" />);

    const listbox = screen.getByRole("listbox");
    expect(within(listbox).getAllByRole("option")).toHaveLength(4);
  });

  it("says so when asked for an overlay it does not have", () => {
    const { container } = render(<Gallery open="nope" />);

    expect(
      container.querySelector('[data-gallery-overlay="unknown"]'),
    ).not.toBeNull();
    expect(screen.getByText("Unknown overlay “nope”.")).toBeInTheDocument();
  });
});
