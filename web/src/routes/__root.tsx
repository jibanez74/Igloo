import { Outlet, createRootRoute, redirect } from "@tanstack/react-router";
import {QueryClientProvider} from '@tanstack/react-query'
import queryClient from '../utils/queryClient';
import AuthProvider from "../context/AuthProvider";
import Navbar from "../components/Navbar";

export const Route = createRootRoute({
  component: () => {
    return (
      <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <Navbar />
        <Outlet />
      </AuthProvider>
      </QueryClientProvider>
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
