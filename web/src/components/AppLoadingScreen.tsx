import { Snowflake } from "lucide-react";
import { useEffect, useState } from "react";
import { MOTION_LOADING_STATE_CLASS } from "@/lib/constants";
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
    <div
      className="fixed inset-0 z-50 overflow-hidden bg-slate-900 text-slate-100"
      role={hasInitialSplash ? undefined : "status"}
      aria-live={hasInitialSplash ? undefined : "polite"}
      aria-atomic={hasInitialSplash ? undefined : "true"}
    >
      <div className="absolute inset-0 bg-slate-950/70" aria-hidden="true" />

      <div className="relative flex min-h-screen items-center justify-center px-4">
        <div className="w-full max-w-md rounded-xl border border-slate-800 bg-slate-900/80 shadow-xl backdrop-blur-sm">
          <div className="px-6 py-8 text-center">
            <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-slate-800">
              <Snowflake
                className={cn("size-5 text-amber-400", MOTION_LOADING_STATE_CLASS)}
                aria-hidden="true"
              />
            </div>
            <p className="text-2xl font-semibold tracking-tight text-slate-100">
              Igloo
            </p>
            <p className="mt-2 text-sm text-slate-400">{message}</p>
          </div>
        </div>
      </div>
    </div>
  );
}
