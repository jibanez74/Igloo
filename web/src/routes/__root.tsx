import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClientProvider } from "@tanstack/solid-query";
import { setAuthState } from "../stores/authStore";
import queryClient from "../utils/queryClient";
import Navbar from "../components/Navbar";

export const Route = createRootRoute({
  beforeLoad: async () => {
    try {
      const res = await fetch("/api/v1/auth/me", {
        credentials: "same-origin",
      });

      const data = await res.json();

      setAuthState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err) {
      console.error(err);
    }
  },
  component: RootLayout,
});

function RootLayout() {
  return (
    <QueryClientProvider client={queryClient}>
      <header>
        <Navbar />
      </header>
      <main>
        <Outlet />
      </main>
    </QueryClientProvider>
  );
}
