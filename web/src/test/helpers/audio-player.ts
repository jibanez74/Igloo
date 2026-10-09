import { vi, type Mock } from "vitest";
import type { AudioPlayerActions, AudioPlayerNowPlaying } from "@/types";

// A stand-in for both audio player hooks, for route and component tests that
// render without an AudioPlayerProvider. Point the hooks at this module:
//
//   vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));
//   vi.mock("@/hooks/useAudioPlayerNowPlaying", () => import("../helpers/audio-player"));
//
// then import the mocks below to assert calls or set what is playing. The
// setup file clears the vi.fn()s after each test; call resetNowPlayingMock()
// in an afterEach when a test changes the now-playing state.

/** Every player action as a vi.fn(), so a new action fails the type check here. */
export const audioPlayerActionsMock = {
  playTrack: vi.fn(),
  playTrackFromList: vi.fn(),
  playQueue: vi.fn(),
  shuffleQueue: vi.fn(),
  extendQueue: vi.fn(),
  startShufflePlayback: vi.fn(),
  startPlayAllPlayback: vi.fn(),
  setTrack: vi.fn(),
  stop: vi.fn(),
  pause: vi.fn(),
  togglePlay: vi.fn(),
  expand: vi.fn(),
  minimize: vi.fn(),
  suspendKeyboard: vi.fn(),
  resumeKeyboard: vi.fn(),
} satisfies { [K in keyof AudioPlayerActions]: Mock<AudioPlayerActions[K]> };

const IDLE: AudioPlayerNowPlaying = {
  currentTrackId: null,
  isPlaying: false,
  isExpanded: false,
};

export const audioPlayerNowPlayingMock: AudioPlayerNowPlaying = { ...IDLE };

export function resetNowPlayingMock() {
  Object.assign(audioPlayerNowPlayingMock, IDLE);
}

export const useAudioPlayerActions = () => audioPlayerActionsMock;
export const useAudioPlayerNowPlaying = () => audioPlayerNowPlayingMock;
