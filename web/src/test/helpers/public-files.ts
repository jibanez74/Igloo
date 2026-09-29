// Readers for committed web/ files (public/, index.html, generated styles)
// that the drift and manifest suites check without a browser. Paths are
// resolved from the vitest cwd, the web/ project root.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

/** Absolute path of a file given relative to web/. */
export const resolveWeb = (relPath: string) => resolve(process.cwd(), relPath);

/** A web/-relative text file's contents. */
export const readWebFile = (relPath: string) =>
  readFileSync(resolveWeb(relPath), "utf8");

/**
 * The width and height of the PNG stream starting at `offset`: the first two
 * IHDR fields, big-endian right after the 8-byte signature and the chunk's
 * length and type.
 */
export function pngDimensions(bytes: Buffer, offset = 0) {
  return {
    width: bytes.readUInt32BE(offset + 16),
    height: bytes.readUInt32BE(offset + 20),
  };
}
