import express from "express";
import errorHandler from "./middleware/error";
import connectToDB from "./lib/db";
import movieRouter from "./routes/movieRoutes";
import jellyfinRouter from "./routes/jellyfinRoutes";

connectToDB();

const app = express();

app.use(express.json());

app.use("/api/v1/movie", movieRouter);
app.use("/api/v1/jellyfin", jellyfinRouter);

app.use(errorHandler);

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
