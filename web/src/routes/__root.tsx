import { redirect, createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClientProvider } from "@tanstack/solid-query";
import { setAuthState } from "../stores/authStore";
import queryClient from "../utils/queryClient";
import Navbar from "../components/Navbar";

export const Route = createRootRoute({
  beforeLoad: async ({ location }) => {
    try {
      const res = await fetch("/api/v1/auth/me", {
        credentials: "same-origin",
      });

      const data = await res.json();

      if (!res.ok) {
        throw redirect({
          to: "/login",
          replace: true,
          search: {
            redirect: location.href,
          },
        });
      }

      setAuthState({
        user: data.user,
        isAuthenticated: true,
        isLoading: false,
      });
    } catch (err) {
      console.error(err);
      throw new Error("unable to check auth state with backend");
    }
  },
  component: RootLayout,
});

function RootLayout() {
  return (
    <QueryClientProvider client={queryClient}>
        <Navbar />
        <main class="pt-16">
          <Outlet />
        </main>
    </QueryClientProvider>
  );
}
