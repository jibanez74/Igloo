import type { ReactNode } from "react";
import MediaNotFound, {
  type BackDestination,
} from "@/components/shared/MediaNotFound";
import { apiErrorMessage } from "@/lib/is-api-failure";

type MediaDetailGuardProps<TPayload> = {
  /** The parsed route id; null when the URL segment was not a number. */
  id: number | null;
  /** Lowercase, for the messages: "movie", "show", "album", "musician". */
  noun: string;
  back: BackDestination;
  isPending: boolean;
  isError: boolean;
  data: { error: boolean; message?: string } | undefined;
  /** The narrowed payload, or null when the response carried none. */
  payload: TPayload | null;
  skeleton: ReactNode;
  /** Receives the id too: past the guard it is known to be a number. */
  children: (payload: TPayload, id: number) => ReactNode;
};

/**
 * The four ways a media detail page can fail to show its subject, in the order
 * every such page checks them: a link that never named one, a request that
 * failed, a request still in flight, and a response that came back empty.
 *
 * Three of the four are dead ends, so each renders `MediaNotFound` with a way
 * out (design-system §3.4) rather than a bare heading. Children take the
 * payload as an argument so it stays narrowed past the guard.
 */
export default function MediaDetailGuard<TPayload>({
  id,
  noun,
  back,
  isPending,
  isError,
  data,
  payload,
  skeleton,
  children,
}: MediaDetailGuardProps<TPayload>) {
  if (id == null) {
    return <MediaNotFound message={`That ${noun} link is not valid.`} back={back} />;
  }

  if (isError || data?.error) {
    return (
      <MediaNotFound
        message={apiErrorMessage(
          data,
          `Failed to load ${noun} details. Please try again later.`,
        )}
        back={back}
      />
    );
  }

  if (isPending) {
    return <>{skeleton}</>;
  }

  if (payload == null) {
    const capitalized = noun.charAt(0).toUpperCase() + noun.slice(1);

    return <MediaNotFound message={`${capitalized} not found.`} back={back} />;
  }

  return <>{children(payload, id)}</>;
}
