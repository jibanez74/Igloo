import { useContext, useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocation, useRouter } from "@tanstack/react-router";
import { AudioPlayerActionsContext } from "@/context/AudioPlayerContext";

/**
 * Whether a navigation is the one useEndSession made: the server session is
 * already gone, so the login route's guard need not ask. History state is
 * untyped here (router-core keeps its own copy of @tanstack/history, so the
 * HistoryState interface cannot be augmented), hence the narrowing.
 */
export function arrivedLoggedOut(state: unknown): boolean {
  return (
    typeof state === "object" &&
    state !== null &&
    "loggedOut" in state &&
    state.loggedOut === true
  );
}

/**
 * Leaves for /login once the server session has ended (logout, account
 * deletion). Playback stops first, so the mini player goes with it, and the
 * login guard skips its auth check for this navigation, so no 401 lands in
 * the cache while the shell still renders. The login page then forgets the
 * session (useForgetEndedSession).
 */
export function useEndSession() {
  const router = useRouter();
  // Read directly rather than through useAudioPlayerActions: where no player
  // is mounted there is simply nothing to stop.
  const audioPlayer = useContext(AudioPlayerActionsContext);

  return async () => {
    audioPlayer?.stop();
    await router.navigate({
      to: "/login",
      replace: true,
      state: (previous) => ({ ...previous, loggedOut: true }),
    });
  };
}

/**
 * On the login page: forgets everything the previous user loaded once
 * useEndSession has brought them here. The navigation resolves before React
 * unmounts the shell, whose queries would rebuild whatever a clear dropped,
 * so it runs from this page's effect, after the shell is gone. It clears the
 * query cache, the mutation cache (which holds password-change variables)
 * and the router's cached matches.
 */
export function useForgetEndedSession() {
  const queryClient = useQueryClient();
  const router = useRouter();
  const loggedOut = useLocation({
    select: (location) => arrivedLoggedOut(location.state),
  });

  useEffect(() => {
    if (!loggedOut) return;
    queryClient.clear();
    router.clearCache();
  }, [loggedOut, queryClient, router]);
}
