import { Outlet, createRootRoute, redirect } from "@tanstack/react-router";
import { QueryClientProvider } from '@tanstack/react-query';
import queryClient from '../utils/queryClient';
import { AuthProvider, useAuth } from "../context/AuthContext";
import Navbar from "../components/Navbar";

const RootComponent = () => {
  const { isAuthenticated } = useAuth();

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        {isAuthenticated && <Navbar />}
        <Outlet />
      </AuthProvider>
    </QueryClientProvider>
  );
};

export const Route = createRootRoute({
  component: RootComponent,
  beforeLoad: ({ location, context }) => {
    const isPublicRoute = ["/login"].includes(location.pathname);
    const isAuthenticated = context.auth?.isAuthenticated;

    if (!isAuthenticated && !isPublicRoute) {
      throw redirect({
        to: "/login",
        search: {
          redirect: location.pathname,
        },
      });
    }

    if (isAuthenticated && isPublicRoute) {
      throw redirect({
        to: "/",
      });
    }
  },
});
