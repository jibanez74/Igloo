import type { ReactNode } from "react";
import MediaNotFound, {
  type BackDestination,
} from "@/components/shared/MediaNotFound";
import { capitalize } from "@/lib/format";

type MediaDetailGuardProps<TPayload> = {
  /** The parsed route id; null when the URL segment was not a number. */
  id: number | null;
  /** Lowercase, for the messages: "movie", "show", "album", "musician". */
  noun: string;
  back: BackDestination;
  isPending: boolean;
  isError: boolean;
  data: { error: boolean; message?: string; status?: number } | undefined;
  /** The narrowed payload, or null when the response carried none. */
  payload: TPayload | null;
  skeleton: ReactNode;
  /** Offered only on the generic failure, the one branch a retry can fix. */
  onRetry?: () => void;
  /** Receives the id too: past the guard it is known to be a number. */
  children: (payload: TPayload, id: number) => ReactNode;
};

/**
 * The four ways a media detail page can fail to show its subject, in the order
 * every such page checks them: a link that never named one, a request that
 * failed, a request still in flight, and a response that came back empty.
 *
 * Three of the four are dead ends, so each renders `MediaNotFound` with a way
 * out (design-system §3.4): a page `h1` naming the failure, one alert sentence
 * and a link back; the failed request alone also offers Try again. The UI owns every sentence here: a 404 reads as the subject being missing, a 403 as the
 * reader not being allowed to see it, and anything else as a load that went
 * wrong, so neither the client's canned "404 - The resource…" string nor a
 * lowercase server constant ("access denied") ever reaches the reader.
 * Children take the payload as an argument so it stays narrowed past the
 * guard.
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
  onRetry,
  children,
}: MediaDetailGuardProps<TPayload>) {
  const notFoundHeading = `${capitalize(noun)} not found`;

  if (id == null) {
    return (
      <MediaNotFound
        heading={notFoundHeading}
        message={`That ${noun} link is not valid.`}
        back={back}
      />
    );
  }

  if (data?.error && data.status === 404) {
    return (
      <MediaNotFound
        heading={notFoundHeading}
        message={`We couldn't find that ${noun}.`}
        back={back}
      />
    );
  }

  if (data?.error && data.status === 403) {
    return (
      <MediaNotFound
        heading="No access"
        message={`You don't have access to this ${noun}.`}
        back={back}
      />
    );
  }

  if (isError || data?.error) {
    return (
      <MediaNotFound
        heading={`Couldn’t load this ${noun}`}
        message={`Something went wrong while loading this ${noun}.`}
        back={back}
        onRetry={onRetry}
      />
    );
  }

  if (isPending) {
    return <>{skeleton}</>;
  }

  if (payload == null) {
    return (
      <MediaNotFound
        heading={notFoundHeading}
        message={`We couldn't find that ${noun}.`}
        back={back}
      />
    );
  }

  return <>{children(payload, id)}</>;
}
