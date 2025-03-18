import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import Navbar from "../components/Navbar";

export const Route = createRootRoute({
  beforeLoad: async () => {
    try {
      const res = await fetch("/api/v1/auth/me", {
        credentials: "include",
      });

      const data = await res.json();

      sessionStorage.setItem("user-igloo", data.user);
    } catch (err) {
      console.error(err);
    }
  },
  component: RootLayout,
});

function RootLayout() {
  const queryClient = new QueryClient();

  

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
