import { createRootRouteWithContext, Outlet } from "@tanstack/solid-router";
import { createQuery, queryOptions } from "@tanstack/solid-query";
import { Show } from "solid-js";
import { setAuthState } from "../stores/authStore";
import Navbar from "../components/Navbar";
import iglooLogo from "../assets/images/logo-alt.png";
import type { QueryClient } from "@tanstack/solid-query";
import type { User } from "../types/User";

type RouterContext = {
  queryClient: QueryClient;
};

const opts = queryOptions({
  queryKey: ["auth"],
  queryFn: async (): Promise<User | null> => {
    try {
      const res = await fetch("/api/v1/auth/me", {
        credentials: "same-origin",
      });

      if (res.ok) {
        const data = await res.json();
        return data.user;
      }

      return null;
    } catch (err) {
      console.error(err);
      return null;
    }
  },
  refetchOnWindowFocus: false,
  staleTime: Infinity,
});

export const Route = createRootRouteWithContext<RouterContext>()({
  loader: ({ context }) => {
    context.queryClient.ensureQueryData(opts);
  },
  component: RootLayout,
});

function RootLayout() {
  const query = createQuery(() => ({
    ...opts,
    onSuccess: (user: User | null) =>
      setAuthState({
        isAuthenticated: !!user,
        user,
      }),
    refetchOnWindowFocus: false,
  }));

  return (
    <>
      <Show
        when={!query.isPending}
        fallback={
          <div class="fixed inset-0 bg-blue-950 flex items-center justify-center z-50 animate-fade-out">
            <div class="text-center">
              <img
                src={iglooLogo}
                alt="Igloo"
                class="h-24 w-auto mx-auto mb-8 animate-bounce"
              />
              <div class="flex items-center justify-center gap-2">
                <div class="w-3 h-3 rounded-full bg-yellow-300 animate-[pulse_1s_ease-in-out_infinite]" />
                <div class="w-3 h-3 rounded-full bg-yellow-300 animate-[pulse_1s_ease-in-out_0.2s_infinite]" />
                <div class="w-3 h-3 rounded-full bg-yellow-300 animate-[pulse_1s_ease-in-out_0.4s_infinite]" />
              </div>
              <p class="mt-4 text-yellow-300 text-lg font-medium">
                Loading your experience...
              </p>
            </div>
          </div>
        }
      >
        <Navbar />
        <main class="pt-16">
          <Outlet />
        </main>
      </Show>
    </>
  );
}
