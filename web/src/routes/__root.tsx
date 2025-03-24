import { createRootRouteWithContext, Outlet } from "@tanstack/solid-router";
import { setAuthState } from "../stores/authStore";
import Navbar from "../components/Navbar";
import type { QueryClient } from "@tanstack/solid-query";

type RouterContext = {
  queryClient: QueryClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async () => {
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
      }
    } catch (err) {
      console.error(err);
      throw new Error("unable to check auth state with backend");
    }
  },
  component: RootLayout,
});

function RootLayout() {
  return (
    <>
      <Navbar />
      <main class="pt-16">
        <Outlet />
      </main>
    </>
  );
}
