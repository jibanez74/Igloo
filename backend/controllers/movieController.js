import { ffmpegInstance } from "../lib/ffmpeg.js";
import fs from "fs";
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

  if (!fs.existsSync(movie.filePath)) {
    return next(
      new ErrorResponse(`File not found at path ${movie.filePath}`, 404)
    );
  }

  const range = req.headers.range;
  const fileSize = movie.mediaContainer.size;

  if (req.query.transcode === "yes") {
    const start = range ? Number(range.replace(/\D/g, "")) : 0;

    res.writeHead(206, {
      "Content-Type": "video/mp4",
      "Accept-Ranges": "bytes",
      "Content-Range": `bytes ${start}-${fileSize - 1}/${fileSize}`,
      "Transfer-Encoding": "chunked",
    });

    const ffmpeg = ffmpegInstance()
      .input(movie.filePath)
      .videoCodec("libx264")
      .audioCodec("aac")
      .format("mp4")
      .outputOptions(["-movflags frag_keyframe+empty_moov+default_base_moof"]);

    if (range) {
      ffmpeg.seekInput((start / fileSize) * movie.mediaContainer.duration);
    }

    ffmpeg
      .on("start", commandLine => {
        console.log("Spawned FFmpeg with command: " + commandLine);
      })
      .on("error", (err, stdout, stderr) => {
        console.error("FFmpeg error:", err.message);
        console.error("FFmpeg stderr:", stderr);
      })
      .on("end", () => {
        console.log("FFmpeg transcoding ended");
      })
      .pipe(res, { end: true });
  } else {
    // Direct streaming logic (unchanged)
    if (!range) {
      return next(new ErrorResponse(`Please provide a range`, 400));
    }

    const chunkSize = 10 ** 6; // 1MB
    const start = Number(range.replace(/\D/g, ""));
    const end = Math.min(start + chunkSize, fileSize - 1);
    const contentLength = end - start + 1;

    const headers = {
      "Content-Range": `bytes ${start}-${end}/${fileSize}`,
      "Accept-Ranges": "bytes",
      "Content-Length": contentLength,
      "Content-Type": `video/${movie.mediaContainer.format}`,
    };

    res.writeHead(206, headers);

    const videoStream = fs.createReadStream(movie.filePath, { start, end });
    videoStream.pipe(res);
  }
});
