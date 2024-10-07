import { createRootRoute, Outlet } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/router-devtools";
import Navbar from "@/components/Navbar";

export const Route = createRootRoute({
  component: Index,
});

function Index() {
  return (
    <>
      <header>
        <Navbar />
      </header>
      <main>
        <Outlet />
      </main>

      <TanStackRouterDevtools />
    </>
  );
}
