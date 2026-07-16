import {
  describePlaybackChannelLayout,
  getPrimaryVideoStream,
} from "@/lib/playback";
import { unwrapStringOrUndefined } from "@/lib/nullable";
import type {
  MediaCapabilityBadge,
  MovieTechnicalDetailsResponse,
} from "@/types/movies";

const HDR_COLOR_TRANSFERS = ["smpte2084", "arib-std-b67"];

/**
 * Derives the source-capability chips (4K / HD, HDR, 5.1 / 7.1, CC) shown in
 * the movie details hero from ffprobe stream metadata. Pure — safe to call
 * with undefined while the technical-details query resolves.
 */
export function deriveMediaCapabilityBadges(
  tech: MovieTechnicalDetailsResponse | undefined,
): MediaCapabilityBadge[] {
  if (!tech) return [];

  const badges: MediaCapabilityBadge[] = [];

  const video = getPrimaryVideoStream(tech.video_streams);
  if (video) {
    // Width thresholds catch anamorphic/scope files whose height falls short
    // of the nominal resolution (e.g. 3840×1600 is still 4K).
    if (video.width >= 3200 || video.height >= 2100) {
      badges.push({ label: "4K", description: "4K Ultra HD video" });
    } else if (video.width >= 1800 || video.height >= 1000) {
      badges.push({ label: "HD", description: "High definition video" });
    }

    const transfer = unwrapStringOrUndefined(video.color_transfer)
      ?.trim()
      .toLowerCase();
    if (transfer && HDR_COLOR_TRANSFERS.includes(transfer)) {
      badges.push({ label: "HDR", description: "High dynamic range video" });
    }
  }

  const surround = (tech.audio_streams ?? [])
    .filter(stream => stream.channels >= 6)
    .sort((a, b) => b.channels - a.channels)[0];
  if (surround) {
    const layout = describePlaybackChannelLayout(
      unwrapStringOrUndefined(surround.channel_layout),
      surround.channels,
    );
    // Only claim a specific layout when ffprobe reported one; 6.1 and other
    // uncommon layouts get the generic badge instead of a guessed 5.1/7.1.
    const label = layout.includes("7.1")
      ? "7.1"
      : layout.includes("5.1")
        ? "5.1"
        : "Surround";
    badges.push({
      label,
      description:
        label === "Surround"
          ? "Surround sound audio"
          : `${label} surround sound audio`,
    });
  }

  if ((tech.subtitles ?? []).length > 0) {
    badges.push({ label: "CC", description: "Subtitles available" });
  }

  return badges;
}
