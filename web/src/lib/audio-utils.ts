// Audio player utility functions
// Shared conversion logic for preparing tracks for the audio player

import type { TrackType, NullableInt64, NullableString } from "@/types";

// Minimal track data needed for audio playback
// This is the common shape that different track types can be converted from
export type PlayableTrackData = {
  id: number;
  title: string;
  file_path: string;
  duration: number;
  codec: string;
  bit_rate: number;
  album_id: NullableInt64;
  musician_id?: NullableInt64;
  album_cover?: NullableString;
  musician_name?: NullableString;
};

// Convert minimal track data to a full TrackType for the audio player
// Fills in default values for fields not needed for playback
export function convertToAudioTrack(track: PlayableTrackData): TrackType {
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
    musician_id: track.musician_id ?? { Int64: 0, Valid: false },
    created_at: "",
    updated_at: "",
  };
}

// Extract cover and musician info from playable track data
export function extractTrackMetadata(track: PlayableTrackData): {
  cover: string | null;
  musician: string | null;
} {
  return {
    cover: track.album_cover?.Valid ? track.album_cover.String : null,
    musician: track.musician_name?.Valid ? track.musician_name.String : null,
  };
}

// Fisher-Yates shuffle algorithm
// Creates a new shuffled array without mutating the original
export function shuffleArray<T>(array: T[]): T[] {
  const shuffled = [...array];
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }
  return shuffled;
}
