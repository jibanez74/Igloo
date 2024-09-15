import { createBrowserRouter } from "react-router-dom";
import RouterLayout from "./RouterLayout";
import LoginPage from "../auth/LoginPage";
import HomePage from "../home/HomePage";
import MoviesPage, { loader as getMovies } from "../movies/MoviesPage";
import MovieDetailsPage from "../movies/MovieDetailsPage";
import PlayMoviePage from "../movies/PlayMovie";

const router = createBrowserRouter([
  {
    path: "/",
    element: <RouterLayout />,
    children: [
      {
        index: true,
        element: <LoginPage />,
      },
      {
        path: "login",
        element: <HomePage />,
      },
      {
        path: "movies",
        children: [
          {
            index: true,
            element: <MoviesPage />,
            loader: getMovies,
          },
          {
            path: ":page",
            element: <MoviesPage />,
            loader: getMovies,
          },
          {
            path: "details/:id",
            element: <MovieDetailsPage />,
          },
          {
            path: "play/:id",
            element: <PlayMoviePage />,
          },
        ],
      },
    ],
  },
]);

export default router;
