import { createBrowserRouter, Navigate } from "react-router-dom";
import RoutesLayout from "./RoutesLayout";
import AuthRoutes from "./AuthRoutes";
import LoginPage from "../auth/LoginPage";
import HomePage from "../home/HomePage";
import MoviesPage from "../movies/MoviesPage";
import MovieDetailsPage from "../movie/MovieDetailsPage";

const routes = createBrowserRouter([
  {
    path: "/",
    element: <RoutesLayout />,
    children: [
      // Protected Routes
      {
        element: <AuthRoutes />,
        children: [
          {
            index: true,
            element: <HomePage />,
          },
          {
            path: "movies",
            children: [
              {
                index: true,
                element: <MoviesPage />,
              },
              {
                path: ":id",
                element: <MovieDetailsPage />,
              },
            ],
          },
        ],
      },

      // Public Routes
      {
        path: "login",
        element: <LoginPage />,
      },

      // Catch all route for 404
      {
        path: "*",
        element: <Navigate to='/' replace />,
      },
    ],
  },
]);

export default routes;
