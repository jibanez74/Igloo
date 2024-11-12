import fs from "fs";
import path from "path";
import asyncHandler from "../lib/asyncHandler";
import Movie from "../models/Movie";
import ErrorResponse from "../lib/errorResponse";
import { ffmpegInstance as ffmpeg } from "../lib/ffmpeg";
import { redis } from "../lib/db";

const ffmpegCommands = new Map();

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

  const size = movie.mediaContainer.size;
  const range = req.headers.range;
  let start = 0;
  let end = size - 1;

  if (range) {
    const parts = range.replace(/bytes=/, "").split("-");
    start = parseInt(parts[0], 10);
    end = parts[1] ? parseInt(parts[1], 10) : size - 1;
  }

  const contentLength = end - start + 1;

  const headers = {
    "Content-Range": `bytes ${start}-${end}/${size}`,
    "Accept-Ranges": "bytes",
    "Content-Length": contentLength,
    "Content-Type": movie.contentType,
  };

  if (req.query.transcode === "yes") {
    headers["Content-Type"] = "video/mp4";
    res.writeHead(206, headers);

    const audioChannels = Number(req.query.channels) || 2;
    const audioCodec = req.query.acodec || "aac";
    const audioBitRate = req.query.abitrate || "196k";
    const videoCodec = req.query.vcodec || "libx264";

    const cmd = ffmpeg(fs.createReadStream(movie.filePath, { start, end }))
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
      .on("start", cmdInfo => {
        console.log(`ffmpeg command started:\n ${err}`);
        ffmpegCommands.set(movieId, command);
      })
      .on("end", () => console.log("ffmpeg process ended"));

    cmd.pipe(res, { end: true });
  } else {
    res.writeHead(206, headers);
    fs.createReadStream(movie.filePath, { start, end }).pipe(res);
  }
});

export const stopProcessing = async (req, res) => {
  const { movieId } = req.params;

  const command = ffmpegCommands.get(movieId);

  if (command) {
    command.kill("SIGKILL");
    ffmpegCommands.delete(movieId);
    res.status(200).json({ message: "Processing stopped" });
  } else {
    res
      .status(404)
      .json({ message: "No active processing found for this movie" });
  }
};
