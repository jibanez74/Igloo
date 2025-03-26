import { createRootRouteWithContext, Outlet } from "@tanstack/solid-router";
import { setAuthState } from "../stores/authStore";
import Navbar from "../components/Navbar";
import type { QueryClient } from "@tanstack/solid-query";

type RouterContext = {
  queryClient: QueryClient;
};

export const Route = createRootRouteWithContext<RouterContext>()({
  beforeLoad: async () => {
    const res = await fetch("/api/v1/auth/me", {
      credentials: "include",
    });

    if (res.ok) {
      const data = await res.json();

      setAuthState({
        isAuthenticated: true,
        user: data.user,
      });
    }
  },
  component: RootLayout,
});

function RootLayout() {
  return (
    <>
      <header>
        <Navbar />
      </header>

      <main class="pt-16">
        <Outlet />
      </main>
    </>
  );
}
