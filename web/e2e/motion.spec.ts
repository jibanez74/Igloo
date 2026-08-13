import { expect, test, type APIResponse, type Page } from "@playwright/test";

import { trackBrowserIssues } from "./e2e-browser-issues";
import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type ComingSoonPage = {
  path: string;
  title: string;
  description: string;
};

const comingSoonPages: ComingSoonPage[] = [
  {
    path: "/photos",
    title: "Photos",
    description:
      "Your personal photo gallery is coming soon. Organize, browse, and share your memories all in one place.",
  },
  {
    path: "/tv-shows",
    title: "TV Shows",
    description:
      "Your TV show library is coming soon. Track episodes, discover new series, and never miss a premiere.",
  },
];

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(page: Page, env: E2EEnv) {
  const loginResponse = await page.context().request.post(
    apiURL(env, "/api/auth/login"),
    {
      data: {
        email: env.email,
        password: env.password,
      },
      failOnStatusCode: false,
    },
  );
  expect(loginResponse.status()).toBe(200);

  const loginBody = await readJSON<unknown>(loginResponse);
  expect(loginBody.error, loginBody.message).toBe(false);
}

async function expectDecorativeAnimationsStopped(page: Page) {
  const decorativeStates = await page.evaluate(() =>
    Array.from(document.querySelectorAll('[data-motion="decorative"]')).map(
      element => {
        const styles = window.getComputedStyle(element);
        const rect = element.getBoundingClientRect();
        const animationNames = styles.animationName
          .split(",")
          .map(name => name.trim());
        const animationDurations = styles.animationDuration
          .split(",")
          .map(duration => duration.trim());

        return {
          animationDurations,
          animationNames,
          visible:
            rect.width > 0 &&
            rect.height > 0 &&
            styles.display !== "none" &&
            styles.visibility !== "hidden",
        };
      },
    ),
  );

  expect(decorativeStates).toHaveLength(4);
  expect(
    decorativeStates.filter(
      state =>
        state.visible &&
        state.animationNames.some((name, index) => {
          if (name === "none") {
            return false;
          }

          return state.animationDurations[index] !== "0s";
        }),
    ),
  ).toEqual([]);
}

// The visible heading, status badge, and description carry the page
// announcement for screen readers; there is deliberately no focusable
// sr-only span, since a non-interactive tab stop is keyboard noise.
async function expectSkipLinkWorks(page: Page) {
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Skip to content" })).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("main")).toBeFocused();
}

test.describe("Reduced motion", () => {
  test("ComingSoon pages keep content visible and stop decorative loops", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const browserIssues = trackBrowserIssues(page);

    await login(page, env);
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.setViewportSize({ width: 1280, height: 900 });

    for (const comingSoonPage of comingSoonPages) {
      await page.goto(apiURL(env, comingSoonPage.path), {
        waitUntil: "networkidle",
      });

      await expect(
        page.getByRole("heading", { name: comingSoonPage.title }),
      ).toBeVisible();
      await expect(page.getByText(comingSoonPage.description)).toBeVisible();
      await expect(page.getByRole("status")).toHaveText(
        "Under Development",
      );
      await expectPageHasNoHorizontalScroll(page);
      await expectDecorativeAnimationsStopped(page);
      await expectSkipLinkWorks(page);
    }

    browserIssues.assertClean();
  });
});
