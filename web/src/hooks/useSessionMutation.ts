// The one module allowed to import useMutation: every other mutation goes
// through useSessionMutation (eslint.config.js bans the direct import).
// eslint-disable-next-line no-restricted-imports
import { useMutation } from "@tanstack/react-query";
import type {
  DefaultError,
  MutationFunctionContext,
  UseMutationOptions,
  UseMutationResult,
} from "@tanstack/react-query";
import { sessionEndedSince, sessionStamp } from "@/hooks/useEndSession";

/** Handed to onMutate so an optimistic write can check after its awaits. */
export type MutationSession = {
  /** Whether the session the mutation started in has ended since. */
  ended: () => boolean;
};

type SessionContext<TOnMutateResult> = {
  session: { session: number };
  result: TOnMutateResult | undefined;
};

export type SessionMutationOptions<
  TData,
  TError,
  TVariables,
  TOnMutateResult,
> = Omit<
  UseMutationOptions<TData, TError, TVariables, TOnMutateResult>,
  "onMutate"
> & {
  onMutate?: (
    variables: TVariables,
    context: MutationFunctionContext,
    session: MutationSession,
  ) => Promise<TOnMutateResult> | TOnMutateResult;
};

// No stamp only when the caller's own onMutate threw, synchronously with the
// mutation's start, so the session is still the one it began in.
function endedFor(wrapped: SessionContext<unknown> | undefined) {
  return wrapped !== undefined && sessionEndedSince(wrapped.session);
}

/**
 * useMutation, fenced to the session it started in.
 *
 * Signing out clears the query and mutation caches, but a request already in
 * flight still resolves, and TanStack still runs its onSuccess / onError /
 * onSettled: they would write the previous account's data into the next
 * account's cache, toast on the login page, or navigate the next reader away
 * from it. This stamps the session when the mutation starts and drops those
 * callbacks once it has ended (useEndSession's sessionStamp /
 * sessionEndedSince).
 *
 * onMutate gets a third argument for the one window the wrapper cannot cover:
 * an optimistic write after `await queryClient.cancelQueries(...)` must return
 * early when `session.ended()`, or it lands in the next account's cache.
 *
 * Per-call `mutate(vars, { onSuccess })` callbacks need no fence: TanStack
 * runs them only while the calling component is mounted, and signing out
 * unmounts the authenticated shell.
 */
export function useSessionMutation<
  TData = unknown,
  TError = DefaultError,
  TVariables = void,
  TOnMutateResult = unknown,
>({
  onMutate,
  onSuccess,
  onError,
  onSettled,
  ...options
}: SessionMutationOptions<
  TData,
  TError,
  TVariables,
  TOnMutateResult
>): UseMutationResult<
  TData,
  TError,
  TVariables,
  SessionContext<TOnMutateResult>
> {
  return useMutation<TData, TError, TVariables, SessionContext<TOnMutateResult>>({
    ...options,
    onMutate: async (variables, context) => {
      // Taken before anything awaits, so a sign-out during onMutate counts.
      const session = sessionStamp();
      const result = await onMutate?.(variables, context, {
        ended: () => sessionEndedSince(session),
      });
      return { session, result };
    },
    onSuccess: (data, variables, wrapped, context) => {
      if (endedFor(wrapped)) return;
      return onSuccess?.(data, variables, wrapped.result as TOnMutateResult, context);
    },
    onError: (error, variables, wrapped, context) => {
      if (endedFor(wrapped)) return;
      return onError?.(error, variables, wrapped?.result, context);
    },
    onSettled: (data, error, variables, wrapped, context) => {
      if (endedFor(wrapped)) return;
      return onSettled?.(data, error, variables, wrapped?.result, context);
    },
  });
}
