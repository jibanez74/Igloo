import express from "express";
import {
  getMovieCount,
  getLatestMovies,
  getMovieByID,
  getMoviesWithPagination,
  createMovie,
  deleteMovie,
  streamMovie,
} from "../controllers/movieController";

const router = express.Router();

router.get("/count", getMovieCount);

router.get("/latest", getLatestMovies);

router.get("/stream/:id", streamMovie);

router.route("/:id").get(getMovieByID).delete(deleteMovie);

router.route("").get(getMoviesWithPagination).post(createMovie);

export default router;
