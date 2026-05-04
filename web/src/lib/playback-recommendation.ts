import type { PlaybackProfileType } from "@/types";

export const HEADROOM_FACTOR = 0.8;

export function recommendedProfileId(
  profiles: PlaybackProfileType[],
  downloadMbps: number | null,
  serverUploadMbps: number | null,
): string | null {
  const dl = downloadMbps ?? Number.POSITIVE_INFINITY;
  const up = serverUploadMbps ?? Number.POSITIVE_INFINITY;
  const effective = Math.min(dl, up);
  if (!Number.isFinite(effective)) return null;

  const cap = effective * HEADROOM_FACTOR;
  const sorted = [...profiles].sort((a, b) => b.video_mbps - a.video_mbps);
  const match = sorted.find(p => p.video_mbps <= cap);
  if (match) return match.id;

  const lowest = sorted[sorted.length - 1];
  return lowest ? lowest.id : null;
}
