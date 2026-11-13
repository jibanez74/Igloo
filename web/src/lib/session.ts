// Which signed-in session this page is in. Module state rather than React
// state: requests and mutation callbacks that outlive a sign-out read it after
// their awaits, long after the components that started them unmounted.

/**
 * How many sessions have ended on this page. Clearing the cache does not stop
 * a mutation that is still in flight: its callbacks run when the response
 * arrives and would write the previous user back into the cache, or over the
 * next one's. Every mutation therefore goes through useSessionMutation, which
 * stamps the session it started in (sessionStamp) and drops its callbacks once
 * that session has ended (sessionEndedSince). Async work outside a mutation
 * that writes the cache or navigates after an await does the same by hand.
 */
let endedSessions = 0;

// From the moment a session starts ending until the login page forgets it, so
// a 401 that lands meanwhile (a burst of refetches, or a request racing a
// logout) does not end it a second time.
let endingSession = false;

export type SessionStamp = { session: number };

export function sessionStamp(): SessionStamp {
  return { session: endedSessions };
}

/**
 * Whether the session a mutation was stamped in has ended since, or is ending:
 * from the moment useEndSession starts leaving for /login, nothing the old
 * session started should toast, write or navigate any more.
 */
export function sessionEndedSince(stamp: SessionStamp | undefined) {
  return endingSession || stamp?.session !== endedSessions;
}

/** useEndSession: the session is ending, from the moment it leaves for /login. */
export function beginEndingSession() {
  endingSession = true;
}

export function isSessionEnding() {
  return endingSession;
}

/** The login page has forgotten the ended session; the next one begins. */
export function forgetEndedSession() {
  endedSessions += 1;
  endingSession = false;
}
