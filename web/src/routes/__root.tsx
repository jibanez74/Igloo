import { onMount } from 'solid-js'
import { createRootRouteWithContext, Outlet } from "@tanstack/solid-router";
import { setAuthState } from "../stores/authStore";
import Navbar from "../components/Navbar";
import type { QueryClient } from "@tanstack/solid-query";

type RouterContext = {
  queryClient: QueryClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
});

function RootLayout() {
  onMount(async () => {
    try {
      const res = await fetch("/api/v1/auth/me", {
        credentials: "same-origin",
      });

      if (res.ok) {
        const data = await res.json();
        setAuthState({
          user: data.user,
          isAuthenticated: true,
          isLoading: false,
        });
      } else {
        setAuthState('isLoading', false);
      }
    } catch (err) {
      console.error(err);
      setAuthState('isLoading', false);
    }
  });

  return  (
    <>
      <Navbar />
      <main class="pt-16">
        <Outlet />
      </main>
    </>
  );
}
