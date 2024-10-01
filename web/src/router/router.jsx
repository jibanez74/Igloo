import { lazy, Suspense } from "react";
import { createBrowserRouter } from "react-router-dom";
import RouterLayout from "./RouterLayout";
import Spinner from "../shared/Spinner";

const HomePage = lazy(() => import("../pages/HomePage"));
const LoginPage = lazy(() => import("../pages/LoginPage"));
const MoviesPage = lazy(() => import("../pages/MoviesPage"));
const MovieDetailsPage = lazy(() => import("../pages/MovieDetailsPage"));

const router = createBrowserRouter([
  {
    path: "/",
    element: <RouterLayout />,
    children: [
      {
        index: true,
        element: (
          <Suspense fallback={<Spinner />}>
            <HomePage />
          </Suspense>
        ),
      },

      {
        path: "login",
        element: (
          <Suspense fallback={<Spinner />}>
            <LoginPage />
          </Suspense>
        ),
      },

      {
        path: "movies",
        children: [
          {
            index: true,
            element: (
              <Suspense fallback={<Spinner />}>
                <MoviesPage />
              </Suspense>
            ),
          },
          {
            path: ":page",
            element: <MoviesPage />,
            loader: getMovies,
          },
          {
            path: "details/:id",
            element: <MovieDetailsPage />,
            loader: getMovie,
          },
        ],
      },
    ],
  },
]);

export default router;
