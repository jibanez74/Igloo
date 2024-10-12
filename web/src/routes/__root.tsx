import { createRootRoute, Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/router-devtools";
import AppContextProvider from "@/AppContext";
import Navbar from "@/components/Navbar";

export const Route = createRootRoute({
  component: Index,
});

function Index() {
  return (
    <AppContextProvider>
      <header>
        <Navbar />
      </header>
      <main>
        <Outlet />
      </main>

      <TanStackRouterDevtools />
    </AppContextProvider>
  );
}
