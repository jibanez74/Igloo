import express from "express";
import errorHandler from "./middleware/error";
import { db } from "./lib/db";
import movieRouter from "./routes/movieRoutes";
import jellyfinRouter from "./routes/jellyfinRoutes";
import { startProcessing, stopProcessing } from './controllers/movieController.js';

await db.connect(process.env.MONGO_URI);

const app = express();

app.use(express.json());

app.use("/api/v1/movie", movieRouter);
app.use("/api/v1/jellyfin", jellyfinRouter);

app.use(errorHandler);

app.post('/api/movies/:movieId/start-processing', startProcessing);
app.post('/api/movies/:movieId/stop-processing', stopProcessing);

const server = app.listen(process.env.PORT, () =>
  console.log(
    `server is running in ${process.env.MODE} mode on port ${process.env.PORT}`
  )
);

process.on("unhandledRejection", (err, promise) => {
  console.log(`Error: ${err.message}`.red);
  // Close server & exit process
  // server.close(() => process.exit(1));
});
