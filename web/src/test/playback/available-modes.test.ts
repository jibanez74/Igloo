import { describe, it, expect } from "vitest";
import { getAvailableModes } from "@/lib/playback";

const modeIds = (modes: ReturnType<typeof getAvailableModes>) =>
  modes.map((m) => m.id);

describe("getAvailableModes container gate", () => {
  it("offers direct play for an eligible MP4 source", () => {
    const ids = modeIds(getAvailableModes(1080, "h264", "aac", "video/mp4"));
    expect(ids).toContain("direct");
    expect(ids).toContain("remux");
  });

  // Audit matrix row 7: MKV must never be offered for direct play — Chrome and
  // Firefox stall silently at 0ms with no MediaError. video/webm and video/ogg
  // are unreachable dead values and must stay refused too.
  it.each([
    "video/x-matroska",
    "video/webm",
    "video/ogg",
    "application/octet-stream",
  ])("refuses direct play for %s while keeping remux", (mimeType) => {
    const ids = modeIds(getAvailableModes(1080, "h264", "aac", mimeType));
    expect(ids).not.toContain("direct");
    expect(ids).toContain("remux");
  });
});
