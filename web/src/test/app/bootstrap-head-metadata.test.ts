import { afterEach, describe, expect, it } from "vitest";
import { syncBootstrapHeadMetadata } from "@/lib/bootstrap-head-metadata";

const BOOTSTRAP_TITLE = "Igloo";
const BOOTSTRAP_DESCRIPTION =
  "Igloo is your personal media center for movies, TV Shows, music, personal videos, photos and so much more. Stream and organize your entire media library.";
const MOVIES_DESCRIPTION =
  "Browse and organize your personal movie collection in your Igloo media library.";
const ORIGINAL_HEAD = document.head.innerHTML;

function setHeadMarkup(markup: string) {
  document.head.innerHTML = markup;
}

afterEach(() => {
  document.head.innerHTML = ORIGINAL_HEAD;
});

describe("syncBootstrapHeadMetadata", () => {
  it("removes bootstrap title and description when route metadata exists", () => {
    setHeadMarkup(`
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title data-igloo-bootstrap-title>${BOOTSTRAP_TITLE}</title>
      <meta
        name="description"
        data-igloo-bootstrap-description
        content="${BOOTSTRAP_DESCRIPTION}"
      />
      <meta name="theme-color" content="#0f172a" />
      <title>Movies - Igloo</title>
      <meta name="description" content="${MOVIES_DESCRIPTION}" />
    `);

    syncBootstrapHeadMetadata();

    expect(
      document.head.querySelector('title[data-igloo-bootstrap-title]'),
    ).toBeNull();
    expect(
      document.head.querySelector(
        'meta[name="description"][data-igloo-bootstrap-description]',
      ),
    ).toBeNull();
    expect(document.head.querySelector("title")?.textContent).toBe(
      "Movies - Igloo",
    );
    expect(
      document.head.querySelector('meta[name="description"]')?.getAttribute(
        "content",
      ),
    ).toBe(MOVIES_DESCRIPTION);
  });

  it("restores bootstrap title and description when route metadata disappears", () => {
    setHeadMarkup(`
      <meta charset="UTF-8" />
      <title data-igloo-bootstrap-title>${BOOTSTRAP_TITLE}</title>
      <meta
        name="description"
        data-igloo-bootstrap-description
        content="${BOOTSTRAP_DESCRIPTION}"
      />
      <title>Movies - Igloo</title>
      <meta name="description" content="${MOVIES_DESCRIPTION}" />
    `);

    syncBootstrapHeadMetadata();
    document.head.querySelector("title:not([data-igloo-bootstrap-title])")?.remove();
    document.head
      .querySelector(
        'meta[name="description"]:not([data-igloo-bootstrap-description])',
      )
      ?.remove();

    syncBootstrapHeadMetadata();

    expect(
      document.head.querySelector('title[data-igloo-bootstrap-title]')
        ?.textContent,
    ).toBe(BOOTSTRAP_TITLE);
    expect(
      document.head
        .querySelector(
          'meta[name="description"][data-igloo-bootstrap-description]',
        )
        ?.getAttribute("content"),
    ).toBe(BOOTSTRAP_DESCRIPTION);
    expect(document.head.querySelector("title")?.textContent).toBe(
      BOOTSTRAP_TITLE,
    );
    expect(
      document.head.querySelector('meta[name="description"]')?.getAttribute(
        "content",
      ),
    ).toBe(BOOTSTRAP_DESCRIPTION);
  });

  it("does not touch unrelated head tags", () => {
    setHeadMarkup(`
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <title data-igloo-bootstrap-title>${BOOTSTRAP_TITLE}</title>
      <meta
        name="description"
        data-igloo-bootstrap-description
        content="${BOOTSTRAP_DESCRIPTION}"
      />
      <meta name="theme-color" content="#0f172a" />
      <link rel="icon" href="/favicon.svg" type="image/svg+xml" />
      <link rel="stylesheet" href="/src/assets/boot.css" />
      <title>Settings - Igloo</title>
      <meta
        name="description"
        content="Configure your Igloo media center settings and preferences."
      />
    `);

    syncBootstrapHeadMetadata();

    expect(
      document.head
        .querySelector('meta[name="viewport"]')
        ?.getAttribute("content"),
    ).toBe("width=device-width, initial-scale=1.0");
    expect(
      document.head
        .querySelector('meta[name="theme-color"]')
        ?.getAttribute("content"),
    ).toBe("#0f172a");
    expect(
      document.head.querySelector('link[rel="icon"]')?.getAttribute("href"),
    ).toBe("/favicon.svg");
    expect(
      document.head
        .querySelector('link[rel="stylesheet"]')
        ?.getAttribute("href"),
    ).toBe("/src/assets/boot.css");
  });
});
