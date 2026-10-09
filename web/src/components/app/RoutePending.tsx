import { Spinner } from "@/components/ui/spinner";

/**
 * The router's default pending view (design-system §3.4): shown in the shell's
 * content area when a route's loader outlasts `pendingMs` and the route has no
 * skeleton of its own, so the sidebar and header stay put. Boot keeps the
 * full-screen `AppLoadingScreen`, which the `_auth` layout sets for itself.
 */
export default function RoutePending() {
  return (
    <div
      className="flex min-h-[50svh] items-center justify-center"
      role="status"
    >
      <div className="flex items-center gap-3 text-muted-foreground">
        <Spinner className="size-5 text-primary" aria-hidden="true" />
        <span>Loading...</span>
      </div>
    </div>
  );
}
