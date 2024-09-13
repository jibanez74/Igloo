import express from "express";
import {
  getMovieCount,
  getMovieByID,
  getMoviesWithPagination,
  createMovie,
} from "../controllers/movieController";

const router = express.Router();

router.get("/count", getMovieCount);

router.route("/:id").get(getMovieByID);

router.route("").get(getMoviesWithPagination).post(createMovie);

export default router;
