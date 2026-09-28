import type { Page, Route } from "@playwright/test";
import type { AuthUser } from "../src/types";
import { authUser } from "../src/test/helpers/fixtures";
import { apiResponse, fulfillIdleScanStatus, fulfillJSON } from "./e2e-api";

// One catch-all `**/api/**` route for the specs that stub every request
// themselves rather than reaching mock-api-server.ts. It answers what the
// authenticated shell asks for on every page, hands the rest to the spec, and
// records anything the spec did not expect so `assertMockSuiteClean` fails.

type MockApiRequest = {
  route: Route;
  url: URL;
  method: string;
};

type MockApiOptions = {
  /** Overrides for the signed-in user. Admins also get the scan-status polls. */
  user?: Partial<AuthUser>;
  /** Answers one request and returns true, or returns false to leave it unexpected. */
  handle?: (request: MockApiRequest) => Promise<boolean>;
};

const PLACEHOLDER_IMAGE_PREFIXES = [
  "/api/tmdb/images/",
  "/api/youtube/thumbnails/",
  "/api/static/",
];

const PLACEHOLDER_IMAGE = `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="240" viewBox="0 0 160 240"><rect width="160" height="240" fill="#f59e0b"/><rect x="14" y="14" width="132" height="212" rx="12" fill="#0f172a"/><circle cx="80" cy="78" r="26" fill="#f8fafc"/><rect x="34" y="146" width="92" height="14" rx="7" fill="#f8fafc"/><rect x="42" y="170" width="76" height="10" rx="5" fill="#fbbf24"/></svg>`;

export async function mockApi(page: Page, { user = {}, handle }: MockApiOptions = {}) {
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (method === "GET") {
      if (PLACEHOLDER_IMAGE_PREFIXES.some(prefix => url.pathname.startsWith(prefix))) {
        await route.fulfill({
          status: 200,
          contentType: "image/svg+xml",
          body: PLACEHOLDER_IMAGE,
        });
        return;
      }

      if (url.pathname === "/api/auth/user") {
        await fulfillJSON(route, authUser(user));
        return;
      }

      if (url.pathname === "/api/notifications/unread-count") {
        await fulfillJSON(route, apiResponse({ unread_count: 0 }));
        return;
      }

      if (user.is_admin && (await fulfillIdleScanStatus(route, url.pathname))) {
        return;
      }
    }

    if (handle && (await handle({ route, url, method }))) {
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
  });

  return { unexpectedApiRequests };
}
