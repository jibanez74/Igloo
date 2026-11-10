import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import ThemeToggle from "@/components/app/ThemeToggle";
import { THEME_COLORS, THEME_STORAGE_KEY } from "@/lib/theme";

function themeColorMeta() {
  return document
    .querySelector('meta[name="theme-color"]')
    ?.getAttribute("content");
}

describe("ThemeToggle", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = "";
    document.head.innerHTML =
      '<meta name="theme-color" content="#000000" />';
  });

  it("toggles the browser theme preference from the top bar", async () => {
    const user = userEvent.setup();

    render(<ThemeToggle />);

    const lightButton = screen.getByRole("button", {
      name: "Switch to light theme",
    });
    // Its name shows in the app's own tooltip, not the browser's.
    expect(lightButton).not.toHaveAttribute("title");

    await user.click(lightButton);

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(themeColorMeta()).toBe(THEME_COLORS.light);

    const darkButton = screen.getByRole("button", {
      name: "Switch to dark theme",
    });
    expect(darkButton).not.toHaveAttribute("title");

    await user.click(darkButton);

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(themeColorMeta()).toBe(THEME_COLORS.dark);
  });

  it("shows its name in a tooltip on keyboard focus, without announcing it twice", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);
    expect(screen.queryByRole("tooltip")).toBeNull();

    await user.tab();

    const button = screen.getByRole("button", { name: "Switch to light theme" });
    expect(button).toHaveFocus();
    expect(screen.getByRole("tooltip")).toHaveTextContent("Switch to light theme");
    expect(button).not.toHaveAttribute("aria-describedby");

    await user.keyboard("{Enter}");

    expect(
      screen.getByRole("button", { name: "Switch to dark theme" }),
    ).toHaveFocus();
  });
});
