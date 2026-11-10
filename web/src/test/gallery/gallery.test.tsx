import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import Gallery from "@/components/gallery/Gallery";
import {
  MOTION_DURATIONS_MS,
  SHADOW_SCALE,
  Z_INDEX_SCALE,
} from "@/lib/theme-tokens";

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
      "tokens",
    ]);
  });

  it("shows a sample of every elevation, layer and duration token", () => {
    const { container } = render(<Gallery />);

    const samples = Array.from(
      container.querySelectorAll<HTMLElement>("[data-token]"),
      sample => sample.dataset.token,
    );
    expect(samples).toEqual([
      ...Object.keys(SHADOW_SCALE).map(name => `shadow-${name}`),
      ...Object.keys(Z_INDEX_SCALE).map(name => `z-${name}`),
      ...Object.keys(MOTION_DURATIONS_MS).map(name => `duration-${name}`),
    ]);
    for (const sample of container.querySelectorAll("[data-token]")) {
      expect(sample).toHaveClass(sample.getAttribute("data-token") ?? "");
    }
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
