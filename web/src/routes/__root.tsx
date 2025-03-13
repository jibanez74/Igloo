import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import Navbar from "../components/Navbar";

export const Route = createRootRoute({
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
