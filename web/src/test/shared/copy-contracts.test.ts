import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * Failure copy names what did not work and stops (design-system §3.4). It
 * neither prescribes a retry, which the Try again control or the user's next
 * press already carries, nor guesses a cause that `apiRequest` cannot know.
 */
const BANNED_FAILURE_PHRASES = [/please try again/i, /check your connection/i];

describe("failure copy contract", () => {
  it("keeps retry advice and guessed causes out of every source file", () => {
    const srcDir = resolve(process.cwd(), "src");
    const sourceFiles = readdirSync(srcDir, {
      recursive: true,
      withFileTypes: true,
    })
      .filter((entry) => entry.isFile() && /\.tsx?$/.test(entry.name))
      .map((entry) => resolve(entry.parentPath, entry.name))
      .filter((path) => !path.startsWith(resolve(srcDir, "test")));

    const violations: string[] = [];
    for (const path of sourceFiles) {
      const name = path.slice(srcDir.length + 1);
      readFileSync(path, "utf8")
        .split("\n")
        .forEach((line, index) => {
          if (BANNED_FAILURE_PHRASES.some((phrase) => phrase.test(line))) {
            violations.push(`${name}:${index + 1}: ${line.trim()}`);
          }
        });
    }

    expect(sourceFiles.length).toBeGreaterThan(100);
    expect(violations).toEqual([]);
  });
});
