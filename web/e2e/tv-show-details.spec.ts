import { expect, test, type Page } from "@playwright/test";
import {
  SHOW_ID,
  seasonEpisodes,
  showDetails,
} from "../src/test/helpers/show-details";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll, tabTo, VIEWPORTS } from "./e2e-layout";
import { apiResponse, fulfillJSON, gateRoute } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";

// The page's content, seasons and URL state are unit-tested
// (src/test/shows/show-details-route.test.tsx and the show component tests);
// this covers what needs a browser: the layout at real widths and the season
// tabs under a real keyboard.

async function mockShowDetailsApi(page: Page) {
  const { unexpectedApiRequests } = await mockApi(page, {
    handle: async ({ route, url, method }) => {
      if (method !== "GET") {
        return false;
      }

      if (url.pathname === `/api/shows/details/${SHOW_ID}`) {
        await fulfillJSON(route, apiResponse(showDetails()));
        return true;
      }

      const episodesMatch = url.pathname.match(
        /^\/api\/shows\/(\d+)\/seasons\/(\d+)\/episodes$/,
      );
      if (episodesMatch) {
        // Validate the show id too: a season fixture served for any id would
        // hide an episode URL built against the wrong show.
        if (Number(episodesMatch[1]) !== SHOW_ID) {
          await fulfillJSON(route, { error: true, message: "show not found" }, 404);
          return true;
        }

        await fulfillJSON(route, apiResponse(seasonEpisodes(Number(episodesMatch[2]))));
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

for (const { label, viewport } of [
  { label: "desktop", viewport: VIEWPORTS.desktop },
  { label: "phone", viewport: VIEWPORTS.phone },
]) {
  test(`show details holds its layout at ${label} width`, async ({ page }) => {
    const browserIssues = trackBrowserIssues(page);
    const unexpectedApiRequests = await mockShowDetailsApi(page);

    await page.setViewportSize(viewport);
    await page.goto(`/tv-shows/${SHOW_ID}`);

    await expect(
      page.getByRole("heading", { level: 1, name: /Frost Harbor/ }),
    ).toBeVisible();
    await expect(
      page.getByRole("list", { name: /Season 1 episodes, 2 in this library/ }),
    ).toBeVisible();

    await expectPageHasNoHorizontalScroll(page);
    assertMockSuiteClean(browserIssues, unexpectedApiRequests);
  });
}

test("season tabs are reachable and operable from the keyboard", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.goto(`/tv-shows/${SHOW_ID}`);

  const season1 = page.getByRole("tab", { name: "Season 1" });
  const season2 = page.getByRole("tab", { name: "Season 2" });
  await expect(season1).toBeVisible();

  // Only the selected tab is in the Tab order (roving tabindex).
  await tabTo(page, season1);

  // Radix tabs move selection with the arrow keys.
  await page.keyboard.press("ArrowRight");
  await expect(season2).toBeFocused();
  await expect(season2).toHaveAttribute("aria-selected", "true");
  await expect(page).toHaveURL(new RegExp(`/tv-shows/${SHOW_ID}\\?season=2$`));

  // The episode list is the tabpanel the selected tab controls.
  const panel = page.getByRole("tabpanel");
  await expect(
    panel.getByRole("list", { name: /Season 2 episodes, 1 in this library/ }),
  ).toBeVisible();
  await expect(season2).toHaveAttribute(
    "aria-controls",
    (await panel.getAttribute("id")) ?? "",
  );

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("a season switch keeps the page while that season's episodes load", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.goto(`/tv-shows/${SHOW_ID}`);
  await expect(
    page.getByRole("list", { name: /Season 1 episodes/ }),
  ).toBeVisible();

  const season2Episodes = await gateRoute(
    page,
    new RegExp(`/api/shows/${SHOW_ID}/seasons/2/episodes$`),
  );
  await page.getByRole("tab", { name: "Season 2" }).click();

  // The loader does not hold the switch: the tab and URL move at once and the
  // episode list shows its own loading state under the unchanged hero.
  await expect(page.getByRole("tab", { name: "Season 2" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByRole("status", { name: "Loading episodes" })).toBeVisible();
  await expect(
    page.getByRole("heading", { level: 1, name: /Frost Harbor/ }),
  ).toBeVisible();

  season2Episodes.release();
  await expect(
    page.getByRole("list", { name: /Season 2 episodes, 1 in this library/ }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
