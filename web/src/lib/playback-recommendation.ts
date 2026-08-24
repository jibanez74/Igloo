import type { PlaybackProfileType } from "@/types";

const HEADROOM_FACTOR = 0.8;

export function recommendedProfileId(
  profiles: readonly PlaybackProfileType[],
  downloadMbps: number | null,
  serverUploadMbps: number | null,
): string | null {
  const dl = downloadMbps ?? Number.POSITIVE_INFINITY;
  const up = serverUploadMbps ?? Number.POSITIVE_INFINITY;
  const effective = Math.min(dl, up);
  if (!Number.isFinite(effective)) return null;

  const cap = effective * HEADROOM_FACTOR;
  let match: PlaybackProfileType | null = null;
  let lowest: PlaybackProfileType | null = null;

  for (const profile of profiles) {
    if (!lowest || profile.video_mbps <= lowest.video_mbps) {
      lowest = profile;
    }

    if (profile.video_mbps > cap) continue;
    if (!match || profile.video_mbps > match.video_mbps) {
      match = profile;
    }
  }

  return (match ?? lowest)?.id ?? null;
}
