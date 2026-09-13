import type { Ref } from "react";
import PlaybackStatusScreen from "@/components/playback/PlaybackStatusScreen";
import type { PlaybackStatus } from "@/types";

type PlaybackStatusViewProps = {
  status: Exclude<PlaybackStatus, { kind: "ready" }>;
  /** What is being played, for the copy: "movie" or "episode". */
  mediaNoun: string;
  onBack: () => void;
  onRetry: () => void;
  backButtonRef: Ref<HTMLButtonElement>;
  containerRef: Ref<HTMLDivElement>;
};

function capitalize(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

export default function PlaybackStatusView({
  status,
  mediaNoun,
  onBack,
  onRetry,
  backButtonRef,
  containerRef,
}: PlaybackStatusViewProps) {
  const backAction = {
    id: "back",
    label: "Back",
    ariaLabel: "Back to previous page",
    icon: "back" as const,
    onClick: onBack,
    buttonRef: backButtonRef,
  };

  switch (status.kind) {
    case "notFound":
      return (
        <PlaybackStatusScreen
          containerRef={containerRef}
          title={`${capitalize(mediaNoun)} not found`}
          message={`The ${mediaNoun} could not be found or you don't have access to it.`}
          actions={[backAction]}
        />
      );
    case "loading":
      return (
        <PlaybackStatusScreen
          variant="loading"
          message={status.message}
        />
      );
    case "modeUnavailable":
      return (
        <PlaybackStatusScreen
          containerRef={containerRef}
          title="Quality not available"
          message={
            <>
              <strong className="text-foreground">{status.modeLabel}</strong> is
              not available for this {mediaNoun}. Go back and choose a different
              quality in Playback Settings.
            </>
          }
          actions={[backAction]}
        />
      );
    case "error":
      return (
        <PlaybackStatusScreen
          containerRef={containerRef}
          title="Playback failed"
          message={status.message}
          actions={[
            {
              id: "retry",
              label: "Try Again",
              ariaLabel: "Try again",
              icon: "retry",
              onClick: onRetry,
            },
            { ...backAction, variant: "secondary" },
          ]}
        />
      );
  }
}
