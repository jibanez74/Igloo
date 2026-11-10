import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { THEME_TOKENS } from "@/lib/theme-tokens";
import { expectNoClass } from "../helpers/dom";

// A menu, a select's list and a popover share one surface, set in the
// primitive (design-system §1.6). Call sites used to pick it, and three
// different ones were in use.

const SURFACE = ["border", "bg-popover", "text-popover-foreground", "shadow-md"];
const OTHER_SURFACES = ["bg-muted", "bg-card", "bg-background"];

describe("the popup surface", () => {
  it("is what a dropdown menu is drawn on", () => {
    render(
      <DropdownMenu open>
        <DropdownMenuTrigger>More options</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Refresh Library</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>,
    );

    const menu = screen.getByRole("menu");
    expect(menu).toHaveClass(...SURFACE);
    expectNoClass(menu, ...OTHER_SURFACES);
  });

  it("is what a select's list is drawn on", () => {
    render(
      <Select open defaultValue="auto">
        <SelectTrigger aria-label="Profile">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="auto">Use recommended</SelectItem>
          <SelectItem value="1080">1080p</SelectItem>
        </SelectContent>
      </Select>,
    );

    const list = screen.getByRole("listbox");
    expect(list).toHaveClass(...SURFACE);
    expectNoClass(list, ...OTHER_SURFACES);
  });

  it("is what a popover is drawn on, whatever size the call site gives it", () => {
    render(
      <Popover open>
        <PopoverTrigger>Notifications</PopoverTrigger>
        <PopoverContent aria-label="Notifications" className="w-80 p-0">
          You're all caught up.
        </PopoverContent>
      </Popover>,
    );

    const popover = screen.getByRole("dialog", { name: "Notifications" });
    expect(popover).toHaveClass(...SURFACE, "w-80", "p-0");
    expectNoClass(popover, ...OTHER_SURFACES, "w-72", "p-4");
  });

  // The item under the pointer or the keyboard focus is filled with `accent`.
  // On a popup of the same color it would not show at all, which is what
  // happened to the menus and lists once drawn on `muted`.
  it.each(["light", "dark"] as const)(
    "is not the color of a highlighted item in the %s theme",
    (theme) => {
      const tokens = THEME_TOKENS[theme];

      expect(tokens["--popover"].hex).toBeTruthy();
      expect(tokens["--accent"].hex).toBeTruthy();
      expect(tokens["--popover"].hex).not.toBe(tokens["--accent"].hex);
    },
  );
});
