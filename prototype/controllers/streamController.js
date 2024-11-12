import fs from "fs";
import path from "path";
import asyncHandler from "../lib/asyncHandler";
import ErrorResponse from "../lib/errorResponse";
import { ffmpegInstance as ffmpeg } from "../lib/ffmpeg";

const ffmpegCommands = new Map();

export const streamVideo = asyncHandler(async (req, res, next) => {
  const filePath = req.query.filepath;

  if (!filePath) {
    return next(new ErrorResponse("file path is required", 400));
  }

  const size = Number(req.query.size);

  if (!size) {
    return next(
      new ErrorResponse("size is required and must be a number", 400)
    );
  }

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
