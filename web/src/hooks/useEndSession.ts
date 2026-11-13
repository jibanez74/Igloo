import { useContext, useEffect, useEffectEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useRouter } from "@tanstack/react-router";
import { AudioPlayerActionsContext } from "@/context/AudioPlayerContext";
import { onSessionExpired } from "@/lib/api";
import {
  beginEndingSession,
  forgetEndedSession,
  isSessionEnding,
} from "@/lib/session";
import { showInfo } from "@/lib/toast-helpers";

/**
 * Whether a navigation arrives because the session ended (useEndSession, or
 * the shell's guard on a 401): the server session is already gone, so the
 * login route's guard need not ask. History state is untyped here
 * (router-core keeps its own copy of @tanstack/history, so the HistoryState
 * interface cannot be augmented), hence the narrowing.
 */
export function arrivedLoggedOut(state: unknown): boolean {
  return (
    typeof state === "object" &&
    state !== null &&
    "loggedOut" in state &&
    state.loggedOut === true
  );
}

type EndSessionOptions = {
  /** Where the login page returns to once the reader signs in again. */
  returnTo?: string;
};

/**
 * Leaves for /login once the server session has ended (logout, account
 * deletion, expiry). Playback stops first, so the mini player goes with it,
 * and the login guard skips its auth check for this navigation, so no 401
 * lands in the cache while the shell still renders. The login page then
 * forgets the session (useForgetEndedSession).
 */
export function useEndSession() {
  const router = useRouter();
  // Read directly rather than through useAudioPlayerActions: where no player
  // is mounted there is simply nothing to stop.
  const audioPlayer = useContext(AudioPlayerActionsContext);

  return async ({ returnTo }: EndSessionOptions = {}) => {
    beginEndingSession();
    audioPlayer?.stop();
    await router.navigate({
      to: "/login",
      search: returnTo === undefined ? {} : { redirect: returnTo },
      replace: true,
      // A page that holds the reader on exit (the video player saving its
      // position) has nothing left to save once the session is gone.
      ignoreBlocker: true,
      state: (previous) => ({ ...previous, loggedOut: true }),
    });
  };
}

/**
 * In the signed-in shell: ends the session when the server says it has
 * ended, a 401 from any request that does not answer one itself
 * (onSessionExpired). Signing in again returns to the page the reader was on.
 * Only the shell listens, so the login page's own expected 401 is ignored.
 */
export function useSignOutOnSessionExpiry() {
  const router = useRouter();
  const endSession = useEndSession();

  const handleExpiry = useEffectEvent(() => {
    if (isSessionEnding()) return;
    showInfo("You've been signed out.");
    void endSession({ returnTo: router.state.location.href });
  });

  useEffect(() => onSessionExpired(() => handleExpiry()), []);
}

/**
 * On the login page: forgets everything the previous user loaded once
 * useEndSession has brought them here. The navigation resolves before React
 * unmounts the shell, whose queries would rebuild whatever a clear dropped,
 * so it runs from this page's effect, after the shell is gone. It ends the
 * session for mutations still in flight, then clears the query cache, the
 * mutation cache (which holds password-change variables) and the router's
 * cached matches.
 */
export function useForgetEndedSession() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const loggedOut = useLocation({
    select: (location) => arrivedLoggedOut(location.state),
  });

  useEffect(() => {
    if (!loggedOut) return;
    forgetEndedSession();
    queryClient.clear();
    router.clearCache();
  }, [loggedOut, queryClient, router]);
}
