import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { WATCH_PROGRESS_MIN_SECONDS } from "@/lib/constants";

// The home "Continue Watching" row is filtered server-side, but the floor it
// filters on is the client's resume-eligibility floor: a title the player
// would not offer to resume must not be offered as in-progress either. The
// queries cannot import a TypeScript constant, so they carry the literal and a
// comment - this test is what keeps the two honest.
// Files are resolved from the vitest cwd (the web/ project root).
const CONTINUE_WATCHING_QUERIES = [
  "../server/sqlc/queries/movie_watch_progress.sql",
  "../server/sqlc/queries/show_episode_watch_progress.sql",
];

const FLOOR_PATTERN = /AND \w+\.progress_sec >= (\d+)/g;

describe("the continue-watching floor matches WATCH_PROGRESS_MIN_SECONDS", () => {
  it.each(CONTINUE_WATCHING_QUERIES)("%s", relPath => {
    const sql = readFileSync(resolve(process.cwd(), relPath), "utf8");
    const floors = [...sql.matchAll(FLOOR_PATTERN)].map(match =>
      Number(match[1]),
    );

    expect(floors).not.toHaveLength(0);
    floors.forEach(floor => expect(floor).toBe(WATCH_PROGRESS_MIN_SECONDS));
  });
});
