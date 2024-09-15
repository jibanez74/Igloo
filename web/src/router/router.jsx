import { createBrowserRouter } from "react-router-dom";
import RouterLayout from "./RouterLayout";
import LoginPage from "../auth/LoginPage";
import HomePage from "../home/HomePage";
import MoviesPage, { loader as getMovies } from "../movies/MoviesPage";
import MovieDetailsPage from "../movies/MovieDetailsPage";
import PlayMoviePage, { loader as getMovie } from "../movies/PlayMovie";

const router = createBrowserRouter([
  {
    path: "/",
    element: <RouterLayout />,
    children: [
      {
        index: true,
        element: <HomePage />,
      },
      {
        path: "login",
        element: <LoginPage />,
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
            loader: getMovie,
          },
        ],
      },
    ],
  },
]);

export default router;
