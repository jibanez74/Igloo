import { unwrapInt, unwrapString } from "@/lib/nullable";
import type { PlayableTrackData } from "@/types";

/**
 * The `TrackItem` props every list derives the same way from an API track row:
 * the library and liked tabs, the search results, the playlist page and its
 * drag overlay. What differs between those lists — the variant, the play
 * handler, whether the actions menu shows — stays at the call site.
 */
export function trackRowProps(track: PlayableTrackData) {
  return {
    id: track.id,
    title: track.title,
    duration: track.duration,
    subtitle: unwrapString(track.musician_name) ?? "Unknown Artist",
    albumId: unwrapInt(track.album_id),
    musicianId: unwrapInt(track.musician_id),
  };
}
