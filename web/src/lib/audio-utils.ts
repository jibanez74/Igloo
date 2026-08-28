import type { PlayableTrackData } from "@/types";

// Start playback, swallowing the rejection browsers throw when autoplay is
// blocked — the UI simply stays paused in that case.
export async function playMediaElement(media: HTMLMediaElement | null) {
  if (!media) return;

  try {
    await media.play();
  } catch {
    // Autoplay can still be blocked by the browser in some cases.
  }
}

export function toggleMediaPlayback(media: HTMLMediaElement | null) {
  if (!media) return;

  if (media.paused) {
    void playMediaElement(media);
  } else {
    media.pause();
  }
}

// Convert minimal track data to a full TrackType for the audio player
// Fills in default values for fields not needed for playback
export function convertToAudioTrack(track: PlayableTrackData) {
  return {
    id: track.id,
    title: track.title,
    sort_title: track.title,
    file_path: track.file_path,
    file_name: "",
    container: "",
    mime_type: "",
    codec: track.codec,
    size: 0,
    track_index: 0,
    duration: track.duration,
    disc: 1,
    channels: "",
    channel_layout: "",
    bit_rate: track.bit_rate,
    profile: "",
    release_date: { String: "", Valid: false },
    year: { Int64: 0, Valid: false },
    composer: { String: "", Valid: false },
    copyright: { String: "", Valid: false },
    language: { String: "", Valid: false },
    album_id: track.album_id,
    musician_id: track.musician_id,
    created_at: "",
    updated_at: "",
  };
}

// Extract cover, musician, and album title info from playable track data
export function extractTrackMetadata(track: PlayableTrackData): {
  cover: string | null;
  musician: string | null;
  albumTitle: string;
} {
  return {
    cover: track.album_cover.Valid ? track.album_cover.String : null,
    musician: track.musician_name.Valid ? track.musician_name.String : null,
    albumTitle: track.album_title?.Valid ? track.album_title.String : "",
  };
}

// Drop repeated ids, keeping the first occurrence. Mixed lists (search
// results, the library tracks tab) and the shuffle endpoint can both repeat an
// id, which would break findIndex-based prev/next navigation.
export function dedupeById<T extends { id: number }>(items: T[]): T[] {
  const seenIds = new Set<number>();

  return items.filter(item => {
    if (seenIds.has(item.id)) {
      return false;
    }
    seenIds.add(item.id);
    return true;
  });
}

// Bound an endless queue's history: keep at most keepBehind tracks before the
// current one and report what was dropped so callers can prune per-track
// metadata. Returns the input array untouched when nothing needs trimming.
export function trimQueueHistory<T extends { id: number }>(
  tracks: T[],
  currentTrackId: number | null,
  keepBehind: number,
): { tracks: T[]; dropped: T[] } {
  const currentIndex =
    currentTrackId === null
      ? -1
      : tracks.findIndex(track => track.id === currentTrackId);
  const dropCount = currentIndex - keepBehind;

  if (dropCount <= 0) {
    return { tracks, dropped: [] };
  }

  return {
    tracks: tracks.slice(dropCount),
    dropped: tracks.slice(0, dropCount),
  };
}

// Shuffle an array using the Fisher-Yates algorithm
export function shuffleArray<T>(array: T[]) {
  const shuffled = [...array];

  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }

  return shuffled;
}
