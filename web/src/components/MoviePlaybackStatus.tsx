import type { Ref } from "react";
import MoviePlaybackStatusScreen from "@/components/MoviePlaybackStatusScreen";
import type { MoviePlaybackStatus } from "@/types";

type PlaybackStatusViewProps = {
  status: Exclude<MoviePlaybackStatus, { kind: "ready" }>;
  onBack: () => void;
  onRetry: () => void;
  backButtonRef: Ref<HTMLButtonElement>;
  containerRef: Ref<HTMLDivElement>;
};

export default function PlaybackStatusView({
  status,
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
        <MoviePlaybackStatusScreen
          containerRef={containerRef}
          title="Movie not found"
          message="The movie could not be found or you don't have access to it."
          actions={[backAction]}
        />
      );
    case "loading":
      return (
        <MoviePlaybackStatusScreen
          variant="loading"
          message={status.message}
        />
      );
    case "modeUnavailable":
      return (
        <MoviePlaybackStatusScreen
          containerRef={containerRef}
          title="Quality not available"
          message={
            <>
              <strong className="text-foreground">{status.modeLabel}</strong> is
              not available for this movie. Go back and choose a different
              quality in Playback Settings.
            </>
          }
          actions={[backAction]}
        />
      );
    case "error":
      return (
        <MoviePlaybackStatusScreen
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
