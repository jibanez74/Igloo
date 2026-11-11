import { expect, test } from "@playwright/test";

import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { loginPageViaApi } from "./e2e-auth";

test.describe("Reduced motion", () => {
  test("Home keeps its content visible without section entrance animations", async ({
    page,
  }) => {
    const browserIssues = trackBrowserIssues(page);

    await loginPageViaApi(page);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1280, height: 900 });

    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Welcome to Igloo" })).toBeVisible();
    await expectPageHasNoHorizontalScroll(page);

    for (const name of [
      "Welcome to Igloo",
      "Recently Added Movies",
      "Recently Added Shows",
      "Recently Added Albums",
      "Now Playing in Theaters",
    ]) {
      const section = page.getByRole("region", { name, exact: true });
      await expect(section).toBeVisible();
      await expect(section).toHaveCSS("animation-name", "none");
      await expect(section).toHaveCSS("opacity", "1");
      await expect(section).toHaveCSS("translate", "none");
    }

    browserIssues.assertClean();
  });
});

// The enter and exit animations are Igloo's own utilities
// (src/assets/animations.css). Screenshots cannot see them: the visual suite
// freezes animations. These read what the browser computed instead, so a
// utility that stops setting its property fails here.
test.describe("Enter animations", () => {
  type Motion = {
    name: string;
    duration: string;
    delay: string;
    fillMode: string;
    timing: string;
    enterOpacity: string;
    enterScale: string;
  };

  const readMotion = (element: Element): Motion => {
    const style = getComputedStyle(element);
    return {
      name: style.animationName,
      duration: style.animationDuration,
      delay: style.animationDelay,
      fillMode: style.animationFillMode,
      timing: style.animationTimingFunction,
      enterOpacity: style.getPropertyValue("--tw-enter-opacity").trim(),
      enterScale: style.getPropertyValue("--tw-enter-scale").trim(),
    };
  };

  test("a section fades in, and a delayed one starts after it", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);

    await loginPageViaApi(page);
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/");

    const welcome = page.getByRole("region", { name: "Welcome to Igloo", exact: true });
    await expect(welcome).toBeVisible();
    expect(await welcome.evaluate(readMotion)).toMatchObject({
      name: "enter",
      duration: "0.2s",
      delay: "0s",
      fillMode: "both",
      timing: "cubic-bezier(0, 0, 0.2, 1)",
      enterOpacity: "0",
    });

    // `delay-75` has to delay the animation, not only transitions: that is
    // the one thing Tailwind's own utility does not do.
    const delayed = page.getByRole("region", { name: "Recently Added Movies", exact: true });
    await expect(delayed).toBeVisible();
    expect(await delayed.evaluate(readMotion)).toMatchObject({
      name: "enter",
      duration: "0.2s",
      delay: "0.075s",
      fillMode: "both",
      enterOpacity: "0",
    });

    browserIssues.assertClean();
  });

  test("a select list and a dialog zoom in from 95%, at their own durations", async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);

    await loginPageViaApi(page);
    await page.setViewportSize({ width: 1280, height: 900 });

    // An anchored popup: the micro duration.
    await page.goto("/settings/playback");
    await page.getByRole("combobox", { name: "Profile" }).click();
    const list = page.locator("[data-slot='select-content']");
    await expect(list).toBeVisible();
    expect(await list.evaluate(readMotion)).toMatchObject({
      name: "enter",
      duration: "0.15s",
      delay: "0s",
      enterOpacity: "0",
      enterScale: ".95",
    });
    await page.keyboard.press("Escape");
    await expect(list).toBeHidden();

    // A modal surface: the standard duration.
    await page.goto("/settings/users");
    await page.getByRole("button", { name: "Add User" }).click();
    const dialog = page.getByRole("dialog", { name: "Add User" });
    await expect(dialog).toBeVisible();
    expect(await dialog.evaluate(readMotion)).toMatchObject({
      name: "enter",
      duration: "0.2s",
      delay: "0s",
      enterOpacity: "0",
      enterScale: ".95",
    });

    browserIssues.assertClean();
  });

  test("the sign-in card rises in over the page duration", async ({ page }) => {
    await page.goto("/login");

    const card = page.locator("[data-slot='card']").first();
    await expect(card).toBeVisible();
    expect(await card.evaluate(readMotion)).toMatchObject({
      name: "enter",
      duration: "0.3s",
      fillMode: "both",
      enterOpacity: "0",
    });
    expect(
      await card.evaluate((element) =>
        getComputedStyle(element).getPropertyValue("--tw-enter-translate-y").trim(),
      ),
    ).not.toBe("0");
  });
});
