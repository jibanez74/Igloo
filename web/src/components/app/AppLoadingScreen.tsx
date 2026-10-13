import BrandMark from "@/components/app/BrandMark";
import { useEffect, useState, type ReactNode } from "react";
import { useRouter, type ErrorComponentProps } from "@tanstack/react-router";
import RetryButton from "@/components/shared/RetryButton";
import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants";
import { ApiFailureError } from "@/lib/is-api-failure";
import { cn } from "@/lib/utils";

type AppLoadingScreenProps = {
  message?: string;
};

export default function AppLoadingScreen({
  message = "Starting Igloo...",
}: AppLoadingScreenProps) {
  const [hasInitialSplash, setHasInitialSplash] = useState(
    () => document.getElementById("initial-splash") !== null,
  );

  useEffect(() => {
    const updateSplashState = () => {
      const hasSplash = document.getElementById("initial-splash") !== null;

      if (!hasSplash) {
        setHasInitialSplash(false);
        observer.disconnect();
      }
    };

    const observer = new MutationObserver(updateSplashState);

    updateSplashState();

    if (document.getElementById("initial-splash") === null) {
      return;
    }

    observer.observe(document.body, { childList: true, subtree: true });

    return () => {
      observer.disconnect();
    };
  }, []);

  return (
    <AppScreenCard
      role={hasInitialSplash ? undefined : "status"}
      aria-live={hasInitialSplash ? undefined : "polite"}
      aria-atomic={hasInitialSplash ? undefined : "true"}
      markClassName={MOTION_LOADING_STATE_CLASS}
    >
      <p className="text-2xl font-semibold tracking-tight text-foreground">
        Igloo
      </p>
      <p className="mt-2 text-sm text-muted-foreground">{message}</p>
    </AppScreenCard>
  );
}

/**
 * The signed-in shell's error boundary, in the boot screen's frame: the
 * shell's auth check could not get an answer (offline, the server down, a
 * proxy error), which says nothing about the session, or a page failed to
 * render. Try again re-runs the check and the page's loaders.
 */
export function AppLoadError({ error }: ErrorComponentProps) {
  const router = useRouter();
  const message =
    error instanceof ApiFailureError
      ? "Couldn’t load Igloo."
      : "Couldn’t load this page.";

  return (
    <AppScreenCard>
      <h1 className="text-2xl font-semibold tracking-tight text-foreground">
        Igloo
      </h1>
      <p role="alert" className="mt-2 text-sm text-muted-foreground">
        {message}
      </p>
      <RetryButton onRetry={() => void router.invalidate()} />
    </AppScreenCard>
  );
}

type AppScreenCardProps = {
  children: ReactNode;
  role?: "status";
  "aria-live"?: "polite";
  "aria-atomic"?: "true";
  markClassName?: string;
};

function AppScreenCard({ children, markClassName, ...liveRegion }: AppScreenCardProps) {
  return (
    <div
      className="fixed inset-0 z-50 overflow-hidden bg-card text-foreground"
      {...liveRegion}
    >
      <div className="absolute inset-0 bg-background/70" aria-hidden="true" />

      <div className="relative flex min-h-screen items-center justify-center px-4">
        <div className="w-full max-w-md rounded-xl border border-border bg-card/80 shadow-xl backdrop-blur-sm">
          <div className="px-6 py-8 text-center">
            <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-muted">
              <BrandMark className={cn("size-6 text-primary", markClassName)} />
            </div>
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
