import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";
import Navbar from "../components/Navbar";

const queryClient = new QueryClient();

export const Route = createRootRoute({
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
