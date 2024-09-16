import fs from "fs";
import path from "path";
import Movie from "../models/Movie";
import { ffmpegInstance as ffmpeg } from "../lib/ffmpeg";

export const getMovieCount = async c => {
  const count = await Movie.countDocuments();

  return c.json({ count }, 200);
};

export const getLatestMovies = async c => {
  const movies = await Movie.find({}, { _id: 1, title: 1, thumb: 1, year: 1 })
    .sort({ createdAt: -1 })
    .limit(12);

  return c.json({ movies }, 200);
};

export const getMovieByID = async c => {
  const id = c.req.params("id");

  if (!id) {
    return c.json({ error: "no id provided" }, 400);
  }

  const movie = await Movie.findById(id)
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
    return c.json({ error: `unable to find movie with id of ${id}` }, 404);
  }

  return c.json({ movie }, 200);
};

export const getMovies = async c => {
  const movies = await Movie.find({}, { _id: 1, title: 1, thumb: 1, year: 1 });

  return c.json({ movies }, 200);
};

// export const streamMovie = asyncHandler(async (req, res, next) => {
//   const movie = await Movie.findById(req.params.id);

//   if (!movie) {
//     return next(
//       new ErrorResponse(`Unable to find movie with id of ${req.params.id}`, 404)
//     );
//   }

//   const file = Bun.file(movie.filePath);

//   const exist = await file.exists();
//   if (!exist) {
//     return next(
//       new ErrorResponse(`File not found at path ${movie.filePath}`, 404)
//     );
//   }

//   if (req.query.transcode === "yes") {
//     console.log("will play movie using ffmpeg");
//     res.writeHead(200, {
//       "Content-Type": "video/mp4",
//     });

//     let ffmpegCommand;

//     const cmd = ffmpeg(fs.createReadStream(movie.filePath))
//       .audioChannels(2)
//       .audioBitrate("196k")
//       .audioCodec("aac")
//       .videoCodec("h264_nvenc")
//       .format("mp4")
//       .outputOptions(["-movflags frag_keyframe+empty_moov"])
//       .on("error", err => {
//         console.error(err);

//         if (!res.headersSent) {
//           res.writeHead(500, { "Content-Type": "text/plain" });
//           res.end("An error occurred while processing the video");
//         }
//       })
//       .on("start", info => {
//         console.log(`ffmpeg command started:\n ${info}`);
//         ffmpegCommand = cmd;
//       })
//       .on("end", () => console.log("ffmpeg process ended"));

//     cmd.pipe(res, { end: true });

//     req.on("close", () => {
//       if (ffmpegCommand) {
//         console.log("Client disconnected, stopping ffmpeg process");
//         ffmpegCommand.kill("SIGKILL");
//       }
//     });
//   } else {
//     const range = req.headers.range;
//     if (!range) {
//       return next(new ErrorResponse(`Please provide a range`, 400));
//     }

//     const fileSize = movie.mediaContainer.size;
//     const chunkSize = 10 ** 6;
//     const start = Number(range.replace(/\D/g, ""));
//     const end = Math.min(start + chunkSize, fileSize - 1);
//     const contentLength = end - start + 1;

//     const headers = {
//       "Content-Range": `bytes ${start}-${end}/${fileSize}`,
//       "Accept-Ranges": "bytes",
//       "Content-Length": contentLength,
//       "Content-Type": movie.contentType,
//     };

//     res.writeHead(206, headers);

//     const videoStream = fs.createReadStream(movie.filePath, { start, end });
//     videoStream.pipe(res);
//   }
// });
