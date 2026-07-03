import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import ThemeToggle from "@/components/ThemeToggle";
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
    expect(lightButton).toHaveAttribute("title", "Switch to light theme");

    await user.click(lightButton);

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
    expect(themeColorMeta()).toBe(THEME_COLORS.light);

    const darkButton = screen.getByRole("button", {
      name: "Switch to dark theme",
    });
    expect(darkButton).toHaveAttribute("title", "Switch to dark theme");

    await user.click(darkButton);

    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
    expect(themeColorMeta()).toBe(THEME_COLORS.dark);
  });
});
