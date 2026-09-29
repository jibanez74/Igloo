import { existsSync, readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { pngDimensions, readWebFile, resolveWeb } from "../helpers/public-files";

// public/ is copied verbatim into the build and embedded in the server, so a
// renamed or resized icon, or a head tag drifting back into index.html, would
// only show up as a broken install or a doubled meta tag.
// "/icon-192.png" as referenced by a page lives at public/icon-192.png.
const publicPath = (href: string) => `public${href}`;

type ManifestIcon = {
  src: string;
  sizes: string;
  type: string;
  purpose?: string;
};

// "192x192", the form a manifest's `sizes` uses.
function pngSize(relPath: string) {
  const { width, height } = pngDimensions(readFileSync(resolveWeb(relPath)));

  return `${width}x${height}`;
}

const manifest = JSON.parse(readWebFile("public/manifest.webmanifest")) as {
  name: string;
  start_url: string;
  display: string;
  icons: ManifestIcon[];
};
const indexHtml = readWebFile("index.html");

describe("web manifest", () => {
  it("names the app and starts it standalone at the root", () => {
    expect(manifest).toMatchObject({
      name: "Igloo",
      start_url: "/",
      display: "standalone",
    });
  });

  it.each(manifest.icons.map(icon => [icon.src, icon] as const))(
    "%s exists and matches its declared size",
    (src, icon) => {
      expect(existsSync(resolveWeb(publicPath(src)))).toBe(true);

      if (icon.type === "image/png") {
        expect(pngSize(publicPath(src))).toBe(icon.sizes);
      }
    },
  );

  it("offers a maskable icon for adaptive launchers", () => {
    expect(manifest.icons.some(icon => icon.purpose === "maskable")).toBe(true);
  });
});

describe("index.html head", () => {
  it("links the manifest, both favicons and a 180px apple touch icon that exist", () => {
    expect(indexHtml).toContain('<link rel="manifest" href="/manifest.webmanifest" />');
    // The ICO carries an explicit size so browsers that read SVG prefer it.
    expect(indexHtml).toContain('<link rel="icon" href="/favicon.ico" sizes="32x32" />');
    expect(indexHtml).toContain(
      '<link rel="icon" href="/favicon.svg" type="image/svg+xml" />',
    );
    expect(existsSync(resolveWeb("public/favicon.ico"))).toBe(true);
    expect(existsSync(resolveWeb("public/favicon.svg"))).toBe(true);
    expect(indexHtml).toContain(
      '<link rel="apple-touch-icon" href="/apple-touch-icon.png" />',
    );
    expect(existsSync(resolveWeb("public/manifest.webmanifest"))).toBe(true);
    expect(pngSize("public/apple-touch-icon.png")).toBe("180x180");
  });

  it("keeps crawlers out with exactly one static robots meta", () => {
    expect(indexHtml.match(/<meta name="robots"[^>]*>/g)).toEqual([
      '<meta name="robots" content="noindex, nofollow" />',
    ]);
  });

  it("leaves the description to the root route's head", () => {
    // A static description would sit next to the route's, doubling the tag.
    expect(indexHtml).not.toMatch(/<meta\s+name="description"/);
  });

  it("keeps a plain pre-boot title fallback", () => {
    expect(indexHtml).toContain("<title>Igloo</title>");
  });
});

describe("robots.txt", () => {
  it("disallows every path", () => {
    expect(readWebFile("public/robots.txt")).toMatch(/^User-agent: \*\nDisallow: \/$/m);
  });
});
