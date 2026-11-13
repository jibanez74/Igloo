import { createContext } from "react";
import type { AudioPlayerActions, AudioPlayerNowPlaying } from "@/types";

// The player's contexts, apart from the provider that fills them: a hook
// that only reads one (useEndSession on the login page) must not pull in the
// player itself, which loads with the signed-in shell (_auth's layout).

export const AudioPlayerActionsContext =
  createContext<AudioPlayerActions | null>(null);

// The queue itself is passed to AudioPlayer as props rather than published as
// a context, so appends to an endless queue re-render only the player. Every
// other subscriber (track rows, the app shell) reads this primitive-only
// slice instead.
export const AudioPlayerNowPlayingContext =
  createContext<AudioPlayerNowPlaying | null>(null);
