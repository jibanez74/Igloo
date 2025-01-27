import { Outlet, createRootRoute, redirect } from "@tanstack/react-router";
import AuthProvider from "@/context/AuthProvider";
import Navbar from "@/components/Navbar";

export const Route = createRootRoute({
  component: () => {
    return (
      <AuthProvider>
        <Navbar />
        <Outlet />
      </AuthProvider>
    );
  },
  beforeLoad: ({ location }) => {
    const isPublicRoute = ["/login"].includes(location.pathname);
    const hasSession = localStorage.getItem("hasSession") === "true";

    if (!hasSession && !isPublicRoute) {
      throw redirect({
        to: "/login",
        search: {
          redirect: location.pathname,
        },
      });
    }

    if (hasSession && isPublicRoute) {
      throw redirect({
        to: "/",
      });
    }
  },
});
