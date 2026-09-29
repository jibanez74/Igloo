import { expect, test } from "@playwright/test";

// Headless Chromium never fetches favicons on its own, so the boot spec's
// "no failed request" net never sees these files. Fetch each linked icon
// and manifest entry explicitly; the server must serve them with an image or
// manifest media type (the Go server sends nosniff, so a wrong type is a
// broken icon, not a warning). Runs in every mode: Vite, the production
// preview and a real instance all serve public/ verbatim.
test("serves every linked icon and the manifest with its media type", async ({
  page,
  request,
}) => {
  await page.goto("/login");

  const hrefs = await page
    .locator(
      'head link[rel="icon"], head link[rel="apple-touch-icon"], head link[rel="manifest"]',
    )
    .evaluateAll(links => links.map(link => link.getAttribute("href")));
  expect(hrefs).toEqual([
    "/favicon.ico",
    "/favicon.svg",
    "/apple-touch-icon.png",
    "/manifest.webmanifest",
  ]);

  const manifestResponse = await request.get("/manifest.webmanifest");
  expect(manifestResponse.status()).toBe(200);
  expect(manifestResponse.headers()["content-type"]).toMatch(
    /^application\/manifest\+json/,
  );
  const manifest = (await manifestResponse.json()) as {
    icons: { src: string }[];
  };
  expect(manifest.icons.length).toBeGreaterThan(0);

  const iconHrefs = [
    ...hrefs.filter((href): href is string => href !== null && href !== "/manifest.webmanifest"),
    ...manifest.icons.map(icon => icon.src),
  ];
  for (const href of iconHrefs) {
    const response = await request.get(href);
    expect(response.status(), href).toBe(200);
    expect(response.headers()["content-type"], href).toMatch(/^image\//);
  }
});
