import { expect, test, type Page, type Route } from "@playwright/test";

type NullableString = {
  String: string;
  Valid: boolean;
};

type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

function nullableString(value = ""): NullableString {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function apiResponse(data: unknown) {
  return {
    error: false,
    data,
  };
}

function track(id: number) {
  return {
    id,
    title: `Track ${id.toString().padStart(4, "0")}`,
    duration: 180,
    codec: "flac",
    bit_rate: 900000,
    file_path: `/music/track-${id}.flac`,
    album_id: nullableInt64(1),
    album_title: nullableString("Mock Album"),
    album_cover: nullableString(),
    musician_id: nullableInt64(1),
    musician_name: nullableString("Mock Artist"),
  };
}

async function fulfillJSON(route: Route, body: unknown) {
  await route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

async function mockMusicApi(page: Page, requestedOffsets: number[]) {
  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Music User",
          email: "music@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/music/stats") {
      await fulfillJSON(route, apiResponse({
        total_albums: 1,
        total_tracks: 2267,
        total_musicians: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/albums") {
      await fulfillJSON(route, apiResponse({
        albums: [],
        total: 0,
        page: 1,
        per_page: 24,
        total_pages: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/musicians") {
      await fulfillJSON(route, apiResponse({
        musicians: [],
        total: 0,
        page: 1,
        per_page: 24,
        total_pages: 1,
      }));
      return;
    }

    if (url.pathname === "/api/music/tracks/liked-ids") {
      await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
      return;
    }

    if (url.pathname === "/api/music/tracks") {
      const limit = Number(url.searchParams.get("limit") ?? "50");
      const offset = Number(url.searchParams.get("offset") ?? "0");
      requestedOffsets.push(offset);

      await fulfillJSON(route, apiResponse({
        tracks: Array.from({ length: limit }, (_, index) => track(offset + index + 1)),
        total: 2267,
        offset,
        limit,
        has_more: offset + limit < 2267,
      }));
      return;
    }

    await fulfillJSON(route, {
      error: true,
      message: `Unexpected API request: ${url.pathname}`,
    });
  });
}

async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(() => {
    const root = document.scrollingElement ?? document.documentElement;
    const offenders = Array.from(document.querySelectorAll<HTMLElement>("body *"))
      .filter(element => {
        const rect = element.getBoundingClientRect();
        return rect.width > 0 && rect.right > window.innerWidth + 1;
      })
      .slice(0, 5)
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        className: element.className.toString(),
        right: element.getBoundingClientRect().right,
      }));

    return {
      clientWidth: root.clientWidth,
      offenders,
      scrollWidth: root.scrollWidth,
    };
  });

  expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1);
  expect(overflow.offenders).toEqual([]);
}

test("tracks tab keeps fetching pages while the virtualized list grows", async ({ page }) => {
  const consoleIssues: string[] = [];
  const requestedOffsets: number[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });

  await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/music?tab=tracks");

  const tracksList = page.getByRole("list", { name: "Tracks" });

  await expect(tracksList).toBeVisible();
  await expect(page.getByText("50 of 2267 tracks loaded")).toBeVisible();
  await expectNoHorizontalOverflow(page);

  for (let index = 0; index < 8; index += 1) {
    await page.evaluate(() => {
      window.scrollTo(0, document.documentElement.scrollHeight);

      for (const element of document.querySelectorAll<HTMLElement>("*")) {
        if (element.scrollHeight > element.clientHeight) {
          element.scrollTop = element.scrollHeight;
        }
      }
    });

    if (requestedOffsets.includes(100)) {
      break;
    }
    await page.waitForTimeout(100);
  }

  await expect.poll(() => requestedOffsets).toContainEqual(100);
  expect(requestedOffsets).toEqual(expect.arrayContaining([0, 50, 100]));

  await page.evaluate(() => {
    window.scrollTo(0, 0);

    for (const element of document.querySelectorAll<HTMLElement>("*")) {
      if (element.scrollTop > 0) {
        element.scrollTop = 0;
      }
    }
  });

  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  expect(consoleIssues).toEqual([]);
});

test("tracks tab fits on mobile", async ({ page }) => {
  const consoleIssues: string[] = [];
  const requestedOffsets: number[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });

  await mockMusicApi(page, requestedOffsets);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/music?tab=tracks");

  await expect(page.getByRole("list", { name: "Tracks" })).toBeVisible();
  await expect(page.getByRole("button", { name: "More actions for Track 0001" })).toBeVisible();
  await expectNoHorizontalOverflow(page);
  expect(consoleIssues).toEqual([]);
});
