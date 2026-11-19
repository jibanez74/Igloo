// The screenshot baselines are drawn in one pinned Playwright image (CI's
// test-visual job and `make test-visual-docker`). Its browser must be the one
// the locked @playwright/test drives, and both places must name the same
// image, or the two would draw different pixels. Fails when they disagree.
//
//   bun scripts/check-playwright-image.ts   (from web/)

import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = join(import.meta.dirname, "..", "..");
const IMAGE =
  /mcr\.microsoft\.com\/playwright:v(\d+\.\d+\.\d+)-noble@sha256:[0-9a-f]{64}/g;

const problems: string[] = [];

const imagesIn = (relativePath: string) => {
  const text = readFileSync(join(root, relativePath), "utf8");
  const found = [...new Set([...text.matchAll(IMAGE)].map(match => match[0]))];
  if (found.length !== 1) {
    problems.push(
      `${relativePath} names ${found.length} pinned Playwright images (want exactly one, tag and digest)`,
    );
  }
  return found;
};

const lockedVersion = (lock: string, name: string) => {
  const escaped = name.replace(/[/@.]/g, character => `\\${character}`);
  const match = lock.match(
    new RegExp(`"${escaped}": \\["${escaped}@(\\d+\\.\\d+\\.\\d+)"`),
  );
  if (!match) problems.push(`web/bun.lock has no locked ${name}`);
  return match?.[1];
};

const workflow = imagesIn(".github/workflows/ci.yml");
const makefile = imagesIn("Makefile");
const lock = readFileSync(join(root, "web", "bun.lock"), "utf8");
const versions = ["@playwright/test", "playwright"].map(name => ({
  name,
  version: lockedVersion(lock, name),
}));

const [image] = workflow;
if (image && makefile[0] && image !== makefile[0]) {
  problems.push(
    `ci.yml pins ${image} but the Makefile pins ${makefile[0]}`,
  );
}
if (image) {
  const tagVersion = image.match(/:v(\d+\.\d+\.\d+)-/)?.[1];
  for (const { name, version } of versions) {
    if (version && version !== tagVersion) {
      problems.push(
        `the pinned image is Playwright ${tagVersion} but web/bun.lock locks ${name} ${version}`,
      );
    }
  }
}

if (problems.length > 0) {
  for (const problem of problems) console.error(`error: ${problem}`);
  console.error(
    "Update the image tag and digest in .github/workflows/ci.yml and the Makefile together with @playwright/test, then redraw the baselines (docs/design-system.md §2.5).",
  );
  process.exit(1);
}
console.log(`Playwright image matches the lock: ${image}`);
