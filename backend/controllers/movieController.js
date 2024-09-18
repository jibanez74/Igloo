import path from "path";
import asyncHandler from "../lib/asyncHandler";
import Movie from "../models/Movie";
import ErrorResponse from "../lib/errorResponse";
import { ffmpegInstance as ffmpeg } from "../lib/ffmpeg";

export const getMovieCount = asyncHandler(async (req, res, next) => {
  const count = await Movie.countDocuments();

  res.status(200).json({
    success: true,
    count,
  });
});

export const getLatestMovies = asyncHandler(async (req, res, next) => {
  const movies = await Movie.find({}, { _id: 1, title: 1, thumb: 1, year: 1 })
    .sort({ createdAt: -1 })
    .limit(12);

  res.status(200).json({
    success: true,
    movies,
  });
});

export const getMovieByID = asyncHandler(async (req, res, next) => {
  const movie = await Movie.findById(req.params.id)
    .populate("genres")
    .populate("studios")
    .populate({
      path: "crewList",
      populate: {
        path: "artist",
        model: "artist",
      },
    })
    .populate({
      path: "castList",
      populate: {
        path: "artist",
        model: "artist",
      },
    });

  if (!movie) {
    return next(
      new ErrorResponse(`unable to find movie with id ${req.params.id}`, 404)
    );
  }

  res.status(200).json({
    success: true,
    movie,
  });
});

export const getMoviesWithPagination = asyncHandler(async (req, res, next) => {
  const pageSize = 24;
  const page = Number(req.query.pageNumber) || 1;

  const keyword = req.query.keyword
    ? {
        title: {
          $regex: req.query.keyword,
          $options: "i",
        },
      }
    : {};

  const count = await Movie.countDocuments({ ...keyword });

  const movies = await Movie.find({ ...keyword })
    .limit(pageSize)
    .skip(pageSize * (page - 1));

  res.status(200).json({
    success: true,
    movies,
    page,
    pages: Math.ceil(count / pageSize),
  });
});

export const createMovie = asyncHandler(async (req, res, next) => {
  const movie = await Movie.create(req.body);

  res.status(201).json({
    success: true,
    movie,
  });
});

export const deleteMovie = asyncHandler(async (req, res) => {
  const movie = await Movie.findById(req.params.id);

  if (!movie) {
    return next(
      new ErrorResponse(`unable movie with id of ${req.params.id}`, 404)
    );
  }

  await movie.remove();

  res.status(200).json({
    success: true,
    message: "movie was deleted successfully",
  });
});

export const streamMovie = asyncHandler(async (req, res, next) => {
  const movie = await Movie.findById(req.params.id);

  if (!movie) {
    return next(
      new ErrorResponse(`Unable to find movie with id of ${req.params.id}`, 404)
    );
  }

  const file = Bun.file(movie.filePath);

  const exist = await file.exists();
  if (!exist) {
    return next(
      new ErrorResponse(`File not found at path ${movie.filePath}`, 404)
    );
  }

  if (req.query.transcode === "yes") {
    res.writeHead(200, {
      "Content-Type": "video/mp4",
    });

    const inputStream = await file.stream();
    const audioChannels = Number(req.query.channels) || 2;
    const audioCodec = req.query.acodec || "aac";
    const audioBitRate = req.query.abitrate || "196k";
    const videoCodec = req.query.vcodec || "libx264";
    let ffmpegProcess = null;

    const cmd = ffmpeg(inputStream)
      .audioChannels(audioChannels)
      .audioBitrate(audioBitRate)
      .audioCodec(audioCodec)
      .videoCodec(videoCodec)
      .format("mp4")
      .outputOptions(["-movflags frag_keyframe+empty_moov"])
      .on("error", err => {
        console.error(err);

        if (!res.headersSent) {
          res.writeHead(500, { "Content-Type": "text/plain" });
          res.end("An error occurred while processing the video");
        }
      })
      .on("start", info => {
        console.log(`ffmpeg command started:\n ${info}`);
        ffmpegProcess = cmd;
      })
      .on("end", () => console.log("ffmpeg process ended"));

    cmd.pipe(res, { end: true });

    req.on("close", () => {
      if (ffmpegProcess) {
        console.log("Client disconnected, stopping ffmpeg process");
        ffmpegProcess.kill("SIGKILL");
      }
    });
  } else {
    const total = movie.mediaContainer.size;
    const range = req.headers.range;

    if (range) {
      const parts = range.replace(/bytes=/, "").split("-");
      const start = parseInt(parts[0], 10);
      const end = parts[1] ? parseInt(parts[1], 10) : total - 1;
      const chunksize = end - start + 1;

      if (
        isNaN(start) ||
        isNaN(end) ||
        start >= total ||
        end >= total ||
        start > end
      ) {
        res.writeHead(416, {
          "Content-Range": `bytes */${total}`,
        });
        return res.end();
      }

      // Ensure start and end are valid numbers
      const validStart = Math.max(0, start);
      const validEnd = Math.min(total - 1, end);

      const readStream = await file.stream({ start: validStart, end: validEnd });

      res.writeHead(206, {
        "Content-Range": `bytes ${validStart}-${validEnd}/${total}`,
        "Accept-Ranges": "bytes",
        "Content-Length": validEnd - validStart + 1,
        "Content-Type": movie.contentType,
      });

      readStream.pipe(res);

      req.on("close", () => {
        readStream.destroy();
      });
    } else {
      const readStream = await file.stream();

      res.writeHead(200, {
        "Content-Length": total,
        "Content-Type": movie.contentType,
        "Accept-Ranges": "bytes",
      });

      readStream.pipe(res);

      req.on("close", () => {
        readStream.destroy();
      });
    }
  }
});
