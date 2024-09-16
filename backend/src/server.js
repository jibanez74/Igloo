import { Hono } from "hono";
import connectToDB from "./lib/db";
import {
  getMovieCount,
  getMovies,
  getMovieByID,
  getLatestMovies,
} from "./handlers/movieController";
import { saveJellyfinMovies } from "./handlers/jellyfinController";

connectToDB();

const app = new Hono();

// jellyfin routes
app.get("/api/v1/jellyfin/transfer", saveJellyfinMovies);

// movie routes
app.get("/api/v1/movie/count", getMovieCount);
app.get("/api/v1/movie/latest", getLatestMovies);
app.get("/api/v1/movie/:id", getMovieByID);
app.get("/api/v1/movie", getMovies);

export default app;
