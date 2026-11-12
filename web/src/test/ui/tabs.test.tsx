import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { expectNoClass } from "../helpers/dom";

// The active tab is a primary pill with the glacier glow under it. The glow
// is the `shadow-glow` token (design-system §1.4), so its strength is set in
// one place for the tab and for every primary button beside it.
describe("TabsTrigger", () => {
  it("lights the active tab with the glow token, not a literal shadow", () => {
    render(
      <Tabs value="all">
        <TabsList>
          <TabsTrigger value="all">All movies</TabsTrigger>
          <TabsTrigger value="genres">Genres</TabsTrigger>
        </TabsList>
      </Tabs>,
    );

    const active = screen.getByRole("tab", { name: "All movies" });
    expect(active).toHaveAttribute("data-state", "active");
    expect(active).toHaveClass(
      "data-[state=active]:bg-primary",
      "data-[state=active]:shadow-glow",
    );
    expectNoClass(
      active,
      "data-[state=active]:shadow-lg",
      "data-[state=active]:shadow-primary/20",
    );

    // One look for every trigger: the state attribute picks the pill.
    const resting = screen.getByRole("tab", { name: "Genres" });
    expect(resting).toHaveAttribute("data-state", "inactive");
    expect(resting).toHaveClass("data-[state=active]:shadow-glow");
  });
});
