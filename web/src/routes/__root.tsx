import { createRootRoute, Outlet } from "@tanstack/solid-router";
import { QueryClient, QueryClientProvider } from "@tanstack/solid-query";

const queryClient = new QueryClient();

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <QueryClientProvider client={queryClient}>
      <header>navbar goes here</header>
      <main>
        <Outlet />
      </main>
    </QueryClientProvider>
  );
}
