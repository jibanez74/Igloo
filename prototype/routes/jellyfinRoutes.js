import express from "express";
import { saveJellyfinMovies } from "../controllers/jellyfinController";

const router = express.Router();

router.get("/transfer", saveJellyfinMovies);

export default router;
